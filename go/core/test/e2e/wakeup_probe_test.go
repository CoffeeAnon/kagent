package e2e_test

import (
	"context"
	"testing"
	"time"

	a2atype "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/kagent-dev/kagent/go/api/gen/kagent/api/v1"
	"github.com/kagent-dev/kagent/go/api/v1alpha3"
	"github.com/kagent-dev/kagent/go/core/internal/substrate"
)

// TestHarnessWakeupProbeTimeout checks that the Harness probe timeout reaches the ActorTemplate.
func TestHarnessWakeupProbeTimeout(t *testing.T) {
	t.Parallel()
	target := interactionTarget(t)
	forEachHarness(t, func(t *testing.T, runtime testHarness) {
		t.Parallel()
		tests := []struct {
			name    string
			timeout *int32
			want    int32
		}{
			{name: "unset keeps the default", timeout: nil, want: 30},
			{name: "configured value reaches the ActorTemplate", timeout: new(int32(90)), want: 90},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				kube := interactionKubeClient(t)
				source := &v1alpha3.Harness{}
				require.NoError(t, kube.Get(t.Context(), ctrlclient.ObjectKey{Namespace: "kagent", Name: runtime.name}, source))
				model := runtime.createModel(t, kube, startInteractionMock(t), nil)
				harness := &v1alpha3.Harness{ObjectMeta: metav1.ObjectMeta{GenerateName: "wakeup-probe-", Namespace: source.Namespace}, Spec: *source.Spec.DeepCopy()}
				harness.Spec.Substrate.WakeupProbeTimeoutSeconds = tc.timeout
				agent := &v1alpha3.Agent{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "wakeup-probe-", Namespace: source.Namespace},
					Spec:       v1alpha3.AgentSpec{Template: v1alpha3.AgentTemplateSource{Inline: &v1alpha3.AgentTemplateSpec{ModelConfig: &v1alpha3.ModelConfigReference{Name: model.Name}, SystemPrompt: "Be helpful."}}},
				}
				require.NoError(t, kube.Create(t.Context(), harness))
				t.Cleanup(func() { require.NoError(t, kube.Delete(context.Background(), harness)) })
				agent.Spec.HarnessRef = corev1.LocalObjectReference{Name: harness.Name}
				require.NoError(t, kube.Create(t.Context(), agent))
				t.Cleanup(func() { require.NoError(t, kube.Delete(context.Background(), agent)) })
				waitForAgentRevision(t, kube, agent)

				ctx, cancel := context.WithTimeout(metadata.AppendToOutgoingContext(t.Context(), "x-user-id", "e2e", a2atype.SvcParamVersion, string(a2atype.Version)), time.Minute)
				t.Cleanup(cancel)
				templateName := agent.Name + "-" + agent.Status.DesiredRevision[:12]
				timeouts := actorTemplateProbeTimeouts(t, ctx, apiv1.NewSystemServiceClient(newControllerConn(t, target)), agent.Namespace, templateName)
				require.Equal(t, []int32{tc.want}, timeouts, "ActorTemplate %s", templateName)
			})
		}
	})
}

// TestSandboxTemplateWakeupProbeTimeout checks that the SandboxTemplate probe timeout reaches the ActorTemplate.
func TestSandboxTemplateWakeupProbeTimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		timeout *int32
		want    int32
	}{
		{name: "unset keeps the default", timeout: nil, want: 30},
		{name: "configured value reaches the ActorTemplate", timeout: new(int32(90)), want: 90},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newSandboxFixture(t)
			kube := interactionKubeClient(t)
			if tc.timeout != nil {
				template := &v1alpha3.SandboxTemplate{}
				key := types.NamespacedName{Namespace: f.template.Namespace, Name: f.template.Name}
				require.NoError(t, kube.Get(f.ctx, key, template))
				template.Spec.Substrate.WakeupProbeTimeoutSeconds = tc.timeout
				require.NoError(t, kube.Update(f.ctx, template))
				require.Eventually(t, func() bool {
					require.NoError(t, kube.Get(f.ctx, key, template))
					return template.Status.ObservedGeneration == template.Generation && meta.IsStatusConditionTrue(template.Status.Conditions, "Ready")
				}, time.Minute, time.Second)
			}
			instance := f.create(t, 5*time.Minute)

			actor, err := findSubstrateActor(f.ctx, f.system, f.template.Namespace, substrate.ActorName(instance.Id))
			require.NoError(t, err)
			require.NotNil(t, actor)
			templateName := actor.GetActorTemplate().GetName()
			require.Equal(t, []int32{tc.want}, actorTemplateProbeTimeouts(t, f.ctx, f.system, f.template.Namespace, templateName), "ActorTemplate %s", templateName)
		})
	}
}

func actorTemplateProbeTimeouts(t *testing.T, ctx context.Context, system apiv1.SystemServiceClient, namespace, name string) []int32 {
	t.Helper()
	summary, err := system.GetSubstrateSummary(ctx, &apiv1.GetSubstrateSummaryRequest{Namespace: namespace, Atespace: namespace})
	require.NoError(t, err)
	require.Empty(t, summary.GetAteApiError())
	var timeouts []int32
	for _, template := range summary.GetActorTemplates() {
		if template.GetMetadata().GetName() == name {
			for _, container := range template.GetContainers() {
				timeouts = append(timeouts, container.GetWakeupProbe().GetTimeoutSeconds())
			}
		}
	}
	return timeouts
}
