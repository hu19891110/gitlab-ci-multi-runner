package common

import (
	"os"
	"testing"

	"errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	s := MockShell{}
	s.On("GetName").Return("script-shell")
	s.On("GenerateScript", mock.Anything, mock.Anything).Return("script", nil)
	RegisterShell(&s)
}

func TestBuildRun(t *testing.T) {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor only once
	p.On("Create").Return(&e).Once()

	// We run everything once
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	e.On("Finish", nil).Return().Once()
	e.On("Cleanup").Return().Once()

	// Run script successfully
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(nil)

	RegisterExecutor("build-run-test", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		GetBuildResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-test",
			},
		},
	}
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestRetryPrepare(t *testing.T) {
	PreparationRetryInterval = 0

	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("Create").Return(&e).Times(3)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("prepare failed")).Twice()
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	e.On("Cleanup").Return().Times(3)

	// Succeed a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(nil)
	e.On("Finish", nil).Return().Once()

	RegisterExecutor("build-run-retry-prepare", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		GetBuildResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-retry-prepare",
			},
		},
	}
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestPrepareFailure(t *testing.T) {
	PreparationRetryInterval = 0

	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("Create").Return(&e).Times(3)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("prepare failed")).Times(3)
	e.On("Cleanup").Return().Times(3)

	RegisterExecutor("build-run-prepare-failure", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		GetBuildResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-prepare-failure",
			},
		},
	}
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "prepare failed")
}

func TestPrepareFailureOnBuildError(t *testing.T) {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("Create").Return(&e).Times(1)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).
		Return(&BuildError{}).Times(1)
	e.On("Cleanup").Return().Times(1)

	RegisterExecutor("build-run-prepare-failure-on-build-error", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		GetBuildResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-prepare-failure-on-build-error",
			},
		},
	}
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.IsType(t, err, &BuildError{})
}

func TestRunFailure(t *testing.T) {
	e := MockExecutor{}
	defer e.AssertExpectations(t)

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("Create").Return(&e).Once()

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup").Return().Once()

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(errors.New("build fail")).Times(3)
	e.On("Finish", errors.New("build fail")).Return().Once()

	RegisterExecutor("build-run-run-failure", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		GetBuildResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-run-failure",
			},
		},
	}

	build.Variables = append(build.Variables, BuildVariable{Key: "PRE_BUILD_ATTEMPTS", Value: "3"})
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "build fail")
}

func TestRunWrongAttempts(t *testing.T) {
	e := MockExecutor{}

	p := MockExecutorProvider{}
	defer p.AssertExpectations(t)

	// Create executor
	p.On("Create").Return(&e)

	// Prepare plan
	e.On("Prepare", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.On("Cleanup").Return()

	// Fail a build script
	e.On("Shell").Return(&ShellScriptInfo{Shell: "script-shell"})
	e.On("Run", mock.Anything).Return(errors.New("Number of attempts out of the range [1, 10]"))
	e.On("Finish", errors.New("Number of attempts out of the range [1, 10]")).Return()

	RegisterExecutor("build-run-attempt-failure", &p)

	successfulBuild, err := GetSuccessfulBuild()
	assert.NoError(t, err)
	build := &Build{
		GetBuildResponse: successfulBuild,
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor: "build-run-attempt-failure",
			},
		},
	}

	build.Variables = append(build.Variables, BuildVariable{Key: "PRE_BUILD_ATTEMPTS", Value: "0"})
	err = build.Run(&Config{}, &Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "Number of attempts out of the range [1, 10]")
}
