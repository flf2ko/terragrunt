package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/shell"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	terragruntDebugFile = "terragrunt-debug.tfvars.json"

	fixtureMultiIncludeDependency = "fixture-multiinclude-dependency"
	fixtureRenderJSON             = "fixture-render-json"
)

var (
	fixtureRenderJSONMainModulePath = filepath.Join(fixtureRenderJSON, "main")
	fixtureRenderJSONDepModulePath  = filepath.Join(fixtureRenderJSON, "dep")
)

func TestDebugGeneratedInputs(t *testing.T) {
	t.Parallel()

	cleanupTerraformFolder(t, TEST_FIXTURE_INPUTS)
	tmpEnvPath := copyEnvironment(t, TEST_FIXTURE_INPUTS)
	rootPath := util.JoinPath(tmpEnvPath, TEST_FIXTURE_INPUTS)

	runTerragrunt(t, fmt.Sprintf("terragrunt plan --terragrunt-non-interactive --terragrunt-debug --terragrunt-working-dir %s", rootPath))

	debugFile := util.JoinPath(rootPath, terragruntDebugFile)
	assert.True(t, util.FileExists(debugFile))

	// If the debug file is generated correctly, we should be able to run terraform apply using the generated var file
	// without going through terragrunt.
	mockOptions, err := options.NewTerragruntOptionsForTest("integration_test")
	require.NoError(t, err)
	mockOptions.WorkingDir = rootPath
	require.NoError(
		t,
		shell.RunTerraformCommand(mockOptions, "apply", "-auto-approve", "-var-file", debugFile),
	)

	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	require.NoError(
		t,
		runTerragruntCommand(t, fmt.Sprintf("terragrunt output -no-color -json --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr),
	)

	outputs := map[string]TerraformOutput{}
	require.NoError(t, json.Unmarshal([]byte(stdout.String()), &outputs))
	validateInputs(t, outputs)

	// Also make sure the undefined variable is not included in the json file
	debugJsonContents, err := ioutil.ReadFile(debugFile)
	require.NoError(t, err)
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal(debugJsonContents, &data))
	_, isDefined := data["undefined_var"]
	assert.False(t, isDefined)
}

func TestTerragruntValidateInputs(t *testing.T) {
	t.Parallel()

	moduleDirs, err := filepath.Glob(filepath.Join("fixture-validate-inputs", "*"))
	require.NoError(t, err)

	for _, module := range moduleDirs {
		// capture range var within range scope so it doesn't change as the tests are spun to the background in the
		// t.Parallel call.
		module := module

		name := filepath.Base(module)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			nameDashSplit := strings.Split(name, "-")
			runTerragruntValidateInputs(t, module, []string{"--terragrunt-strict-validate"}, nameDashSplit[0] == "success")
		})
	}
}

func TestTerragruntValidateInputsWithCLIVars(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join("fixture-validate-inputs", "fail-no-inputs")
	args := []string{"-var=input=from_env"}
	runTerragruntValidateInputs(t, moduleDir, args, true)
}

func TestTerragruntValidateInputsWithCLIVarFile(t *testing.T) {
	t.Parallel()

	curdir, err := os.Getwd()
	require.NoError(t, err)

	moduleDir := filepath.Join("fixture-validate-inputs", "fail-no-inputs")
	args := []string{fmt.Sprintf("-var-file=%s/fixture-validate-inputs/success-var-file/varfiles/main.tfvars", curdir)}
	runTerragruntValidateInputs(t, moduleDir, args, true)
}

func TestTerragruntValidateInputsWithStrictMode(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join("fixture-validate-inputs", "success-inputs-only")
	args := []string{"--terragrunt-strict-validate"}
	runTerragruntValidateInputs(t, moduleDir, args, true)
}

func TestTerragruntValidateInputsWithStrictModeDisabledAndUnusedVar(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join("fixture-validate-inputs", "success-inputs-only")
	args := []string{"-var=testvariable=testvalue"}
	runTerragruntValidateInputs(t, moduleDir, args, true)
}

func TestTerragruntValidateInputsWithStrictModeEnabledAndUnusedVar(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join("fixture-validate-inputs", "success-inputs-only")
	args := []string{"-var=testvariable=testvalue", "--terragrunt-strict-validate"}
	runTerragruntValidateInputs(t, moduleDir, args, false)
}

func TestTerragruntValidateInputsWithStrictModeEnabledAndUnusedInputs(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join("fixture-validate-inputs", "fail-unused-inputs")
	args := []string{"--terragrunt-strict-validate"}
	runTerragruntValidateInputs(t, moduleDir, args, false)
}

func TestTerragruntValidateInputsWithStrictModeDisabledAndUnusedInputs(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join("fixture-validate-inputs", "fail-unused-inputs")
	args := []string{}
	runTerragruntValidateInputs(t, moduleDir, args, true)
}

func TestRenderJSONConfig(t *testing.T) {
	t.Parallel()

	tmpDir, err := ioutil.TempDir("", "terragrunt-render-json-*")
	require.NoError(t, err)
	jsonOut := filepath.Join(tmpDir, "terragrunt_rendered.json")
	defer os.RemoveAll(tmpDir)

	cleanupTerraformFolder(t, fixtureRenderJSONMainModulePath)
	cleanupTerraformFolder(t, fixtureRenderJSONDepModulePath)

	runTerragrunt(t, fmt.Sprintf("terragrunt run-all apply -auto-approve --terragrunt-non-interactive --terragrunt-log-level debug --terragrunt-working-dir %s", fixtureRenderJSON))
	runTerragrunt(t, fmt.Sprintf("terragrunt render-json --terragrunt-non-interactive --terragrunt-log-level debug --terragrunt-working-dir %s --terragrunt-json-out %s", fixtureRenderJSONMainModulePath, jsonOut))

	jsonBytes, err := ioutil.ReadFile(jsonOut)
	require.NoError(t, err)

	var rendered map[string]interface{}
	require.NoError(t, json.Unmarshal(jsonBytes, &rendered))

	// Make sure all terraform block is visible
	terraformBlock, hasTerraform := rendered["terraform"]
	if assert.True(t, hasTerraform) {
		source, hasSource := terraformBlock.(map[string]interface{})["source"]
		require.True(t, hasSource)
		assert.Equal(t, "./module", source)
	}

	// Make sure included remote_state is rendered out
	remote_state, hasRemoteState := rendered["remote_state"]
	if assert.True(t, hasRemoteState) {
		assert.Equal(
			t,
			map[string]interface{}{
				"backend": "local",
				"generate": map[string]interface{}{
					"path":      "backend.tf",
					"if_exists": "overwrite_terragrunt",
				},
				"config": map[string]interface{}{
					"path": "foo.tfstate",
				},
				"disable_init":                    false,
				"disable_dependency_optimization": false,
			},
			remote_state.(map[string]interface{}),
		)
	}

	// Make sure included dependency block is rendered out
	generateBlocks, hasGenerate := rendered["generate"]
	if assert.True(t, hasGenerate) {
		assert.Equal(
			t,
			map[string]interface{}{
				"provider": map[string]interface{}{
					"path":              "provider.tf",
					"comment_prefix":    "# ",
					"disable_signature": false,
					"if_exists":         "overwrite_terragrunt",
					"contents": `provider "aws" {
  region = "us-east-1"
}
`,
				},
			},
			generateBlocks.(map[string]interface{}),
		)
	}

	// Make sure all inputs are merged together
	inputsBlock, hasInputs := rendered["inputs"]
	if assert.True(t, hasInputs) {
		assert.Equal(
			t,
			map[string]interface{}{
				"env":        "qa",
				"name":       "dep",
				"type":       "main",
				"aws_region": "us-east-1",
			},
			inputsBlock.(map[string]interface{}),
		)
	}
}

func TestDependencyGraphWithMultiInclude(t *testing.T) {
	t.Parallel()

	cleanupTerraformFolder(t, fixtureMultiIncludeDependency)
	tmpEnvPath := copyEnvironment(t, fixtureMultiIncludeDependency)
	rootPath := util.JoinPath(tmpEnvPath, fixtureMultiIncludeDependency)

	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	require.NoError(
		t,
		runTerragruntCommand(t, fmt.Sprintf("terragrunt graph-dependencies --terragrunt-non-interactive --terragrunt-working-dir %s", rootPath), &stdout, &stderr),
	)
	stdoutStr := stdout.String()

	assert.Contains(t, stdoutStr, `"main" -> "depa";`)
	assert.Contains(t, stdoutStr, `"main" -> "depb";`)
	assert.Contains(t, stdoutStr, `"main" -> "depc";`)
}

func runTerragruntValidateInputs(t *testing.T, moduleDir string, extraArgs []string, isSuccessTest bool) {
	maybeNested := filepath.Join(moduleDir, "module")
	if util.FileExists(maybeNested) {
		// Nested module test case with included file, so run terragrunt from the nested module.
		moduleDir = maybeNested
	}

	cmd := fmt.Sprintf("terragrunt validate-inputs %s --terragrunt-log-level debug --terragrunt-non-interactive --terragrunt-working-dir %s", strings.Join(extraArgs, " "), moduleDir)
	t.Logf("Command: %s", cmd)
	_, _, err := runTerragruntCommandWithOutput(t, cmd)
	if isSuccessTest {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
	}
}
