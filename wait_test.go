package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/kayac/ecspresso/v2"
)

func TestLifecycleStageIndex(t *testing.T) {
	// The waiter compares positions instead of equality, so the ordering of the
	// stages within the deployment lifecycle is what matters.
	stages := types.ServiceDeploymentLifecycleStage("").Values()
	for i, stage := range stages {
		if got := ecspresso.LifecycleStageIndex(stage); got != i {
			t.Errorf("lifecycleStageIndex(%s) = %d, expected %d", stage, got, i)
		}
	}

	if scaleUp, bakeTime := ecspresso.LifecycleStageIndex(types.ServiceDeploymentLifecycleStageScaleUp),
		ecspresso.LifecycleStageIndex(types.ServiceDeploymentLifecycleStageBakeTime); scaleUp >= bakeTime {
		t.Errorf("SCALE_UP (%d) must come before BAKE_TIME (%d)", scaleUp, bakeTime)
	}

	// A rolling deployment reports no lifecycle stage, which must never satisfy
	// a target stage.
	for _, stage := range []types.ServiceDeploymentLifecycleStage{"", "NO_SUCH_STAGE"} {
		if got := ecspresso.LifecycleStageIndex(stage); got != -1 {
			t.Errorf("lifecycleStageIndex(%q) = %d, expected -1", stage, got)
		}
	}
}

func TestValidateLifecycleStageSupported(t *testing.T) {
	stage := types.ServiceDeploymentLifecycleStageBakeTime

	for _, strategy := range []types.DeploymentStrategy{
		types.DeploymentStrategyBlueGreen,
		types.DeploymentStrategyLinear,
		types.DeploymentStrategyCanary,
		"",
	} {
		sv := &ecspresso.Service{
			Service: types.Service{
				DeploymentConfiguration: &types.DeploymentConfiguration{Strategy: strategy},
			},
		}
		if err := ecspresso.ValidateLifecycleStageSupported(sv, stage); err != nil {
			t.Errorf("strategy %q should be supported, got %v", strategy, err)
		}
	}

	// ROLLING never reports a lifecycle stage, so waiting for one would block
	// until the timeout expires. Fail fast instead.
	sv := &ecspresso.Service{
		Service: types.Service{
			DeploymentConfiguration: &types.DeploymentConfiguration{
				Strategy: types.DeploymentStrategyRolling,
			},
		},
	}
	if err := ecspresso.ValidateLifecycleStageSupported(sv, stage); err == nil {
		t.Error("ROLLING strategy should not be supported")
	}

	if err := ecspresso.ValidateLifecycleStageSupported(nil, stage); err != nil {
		t.Errorf("nil service should be allowed, got %v", err)
	}
}

func TestWaitFuncForECSLifecycleStage(t *testing.T) {
	app := &ecspresso.App{}
	sv := &ecspresso.Service{
		Service: types.Service{
			DeploymentController: &types.DeploymentController{
				Type: types.DeploymentControllerTypeEcs,
			},
		},
	}

	if _, err := app.WaitFunc(sv, nil, "ecs:BAKE_TIME"); err != nil {
		t.Errorf("ecs:BAKE_TIME should be supported by the ECS deployment controller, got %v", err)
	}
	if _, err := app.WaitFunc(sv, nil, "ecs:CLEAN_UP"); err != nil {
		t.Errorf("ecs:CLEAN_UP should be supported by the ECS deployment controller, got %v", err)
	}

	// The lifecycle stage of an ECS deployment has no meaning for CodeDeploy.
	cd := &ecspresso.Service{
		Service: types.Service{
			DeploymentController: &types.DeploymentController{
				Type: types.DeploymentControllerTypeCodeDeploy,
			},
		},
	}
	if _, err := app.WaitFunc(cd, nil, "ecs:BAKE_TIME"); err == nil {
		t.Error("ecs:* should not be accepted for the CodeDeploy deployment controller")
	}
}
