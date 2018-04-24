// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// TestEmptyNodeJS simply tests that we can run an empty NodeJS project.
func TestEmptyNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("empty", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

// TestEmptyPython simply tests that we can run an empty Python project.
func TestEmptyPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:   filepath.Join("empty", "python"),
		Quick: true,
	})
}

// TestProjectMain tests out the ability to override the main entrypoint.
func TestProjectMain(t *testing.T) {
	var test integration.ProgramTestOptions
	test = integration.ProgramTestOptions{
		Dir:          "project_main",
		Dependencies: []string{"@pulumi/pulumi"},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Simple runtime validation that just ensures the checkpoint was written and read.
			assert.NotNil(t, stackInfo.Deployment)
		},
	}
	integration.ProgramTest(t, &test)

	t.Run("Error_AbsolutePath", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.ImportDirectory("project_main_abs")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "main-abs")
		stdout, stderr := e.RunCommandExpectError("pulumi", "update",  "--no-interactive", "--force")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "project 'main' must be a relative path")
		e.RunCommand("pulumi", "stack", "rm", "--yes")
	})

	t.Run("Error_ParentFolder", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.ImportDirectory("project_main_parent")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "main-parent")
		stdout, stderr := e.RunCommandExpectError("pulumi", "update",  "--no-interactive", "--force")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "project 'main' must be a subfolder")
		e.RunCommand("pulumi", "stack", "rm", "--yes")
	})
}

// TestStackProjectName ensures we can read the Pulumi stack and project name from within the program.
func TestStackProjectName(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_project_name",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
	})
}

// TestStackTagValidation verifies various error scenarios related to stack names and tags.
func TestStackTagValidation(t *testing.T) {
	t.Run("Error_StackName", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.RunCommand("git", "init")
		e.RunCommand("pulumi", "init")

		e.ImportDirectory("stack_project_name")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "invalid name (spaces, parens, etc.)")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "error: could not create stack:")
		assert.Contains(t, stderr, "validating stack properties:")
		assert.Contains(t, stderr, "stack name can only contain alphanumeric, hyphens, or underscores")
	})

	t.Run("Error_ProjectName", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.RunCommand("git", "init")
		e.RunCommand("pulumi", "init")

		e.ImportDirectory("stack_project_name")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

		// Change the contents of the Name property of Pulumi.yaml.
		yamlPath := path.Join(e.CWD, "Pulumi.yaml")
		err := integration.ReplaceInFile("name: stack_project_name", "name: something using forbidden characters!", yamlPath)
		assert.NoError(t, err)

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "valid-name")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "error: could not create stack:")
		assert.Contains(t, stderr, "validating stack properties:")
		assert.Contains(t, stderr, "project name can only contain alphanumeric, hyphens, or underscores")
	})

	t.Run("Error_DescriptionLength", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.RunCommand("git", "init")
		e.RunCommand("pulumi", "init")

		e.ImportDirectory("stack_project_name")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

		prefix := "lorem ipsum dolor sit amet"     // 26
		prefix = prefix + prefix + prefix + prefix // 104
		prefix = prefix + prefix + prefix + prefix // 416 + the current Pulumi.yaml's description

		// Change the contents of the Description property of Pulumi.yaml.
		yamlPath := path.Join(e.CWD, "Pulumi.yaml")
		err := integration.ReplaceInFile("description: ", "description: "+prefix, yamlPath)
		assert.NoError(t, err)

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "valid-name")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "error: could not create stack:")
		assert.Contains(t, stderr, "validating stack properties:")
		assert.Contains(t, stderr, "stack tag \"pulumi:description\" value is too long (max length 255 characters)")
	})
}

// TestStackOutputs ensures we can export variables from a stack and have them get recorded as outputs.
func TestStackOutputs(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_outputs",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Ensure the checkpoint contains a single resource, the Stack, with two outputs.
			fmt.Printf("Deployment: %v", stackInfo.Deployment)
			assert.NotNil(t, stackInfo.Deployment)
			if assert.Equal(t, 1, len(stackInfo.Deployment.Resources)) {
				stackRes := stackInfo.Deployment.Resources[0]
				assert.NotNil(t, stackRes)
				assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
				assert.Equal(t, 0, len(stackRes.Inputs))
				assert.Equal(t, 2, len(stackRes.Outputs))
				assert.Equal(t, "ABC", stackRes.Outputs["xyz"])
				assert.Equal(t, float64(42), stackRes.Outputs["foo"])
			}
		},
	})
}

// TestStackParenting tests out that stacks and components are parented correctly.
func TestStackParenting(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_parenting",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			// Ensure the checkpoint contains resources parented correctly.  This should look like this:
			//
			//     A      F
			//    / \      \
			//   B   C      G
			//      / \
			//     D   E
			//
			// with the caveat, of course, that A and F will share a common parent, the implicit stack.

			assert.NotNil(t, stackInfo.Deployment)
			if assert.Equal(t, 8, len(stackInfo.Deployment.Resources)) {
				stackRes := stackInfo.Deployment.Resources[0]
				assert.NotNil(t, stackRes)
				assert.Equal(t, resource.RootStackType, stackRes.Type)
				assert.Equal(t, "", string(stackRes.Parent))

				urns := make(map[string]resource.URN)
				for _, res := range stackInfo.Deployment.Resources[1:] {
					assert.NotNil(t, res)

					urns[string(res.URN.Name())] = res.URN
					switch res.URN.Name() {
					case "a", "f":
						assert.NotEqual(t, "", res.Parent)
						assert.Equal(t, stackRes.URN, res.Parent)
					case "b", "c":
						assert.Equal(t, urns["a"], res.Parent)
					case "d", "e":
						assert.Equal(t, urns["c"], res.Parent)
					case "g":
						assert.Equal(t, urns["f"], res.Parent)
					default:
						t.Fatalf("unexpected name %s", res.URN.Name())
					}
				}
			}
		},
	})
}

// TestStackDependencyGraph tests that the dependency graph of a stack is saved
// in the checkpoint file.
func TestStackDependencyGraph(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_dependencies",
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			assert.NotNil(t, stackInfo.Deployment)
			latest := stackInfo.Deployment
			assert.True(t, len(latest.Resources) >= 2)
			fmt.Println(latest.Resources)
			sawFirst := false
			sawSecond := false
			for _, res := range latest.Resources {
				urn := string(res.URN)
				if strings.Contains(urn, "dynamic:Resource::first") {
					// The first resource doesn't depend on anything.
					assert.Equal(t, 0, len(res.Dependencies))
					sawFirst = true
				} else if strings.Contains(urn, "dynamic:Resource::second") {
					// The second resource uses an Output property of the first resource, so it
					// depends directly on first.
					assert.Equal(t, 1, len(res.Dependencies))
					assert.True(t, strings.Contains(string(res.Dependencies[0]), "dynamic:Resource::first"))
					sawSecond = true
				}
			}

			assert.True(t, sawFirst && sawSecond)
		},
	})
}

// TestConfigSave ensures that config commands in the Pulumi CLI work as expected.
func TestConfigSave(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

	// Initialize an empty stack.
	path := filepath.Join(e.RootPath, "Pulumi.yaml")
	err := (&workspace.Project{
		Name:    "testing-config",
		Runtime: "nodejs",
	}).Save(path)
	assert.NoError(t, err)
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "testing-2")
	e.RunCommand("pulumi", "stack", "init", "testing-1")

	// Now configure and save a few different things:
	e.RunCommand("pulumi", "config", "set", "configA", "value1")
	e.RunCommand("pulumi", "config", "set", "configB", "value2", "--stack", "testing-2")

	e.RunCommand("pulumi", "stack", "select", "testing-2")

	e.RunCommand("pulumi", "config", "set", "configD", "value4")
	e.RunCommand("pulumi", "config", "set", "configC", "value3", "--stack", "testing-1")

	// Now read back the config using the CLI:
	{
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configB")
		assert.Equal(t, "value2\n", stdout)
	}
	{
		// the config in a different stack, so this should error.
		stdout, stderr := e.RunCommandExpectError("pulumi", "config", "get", "configA")
		assert.Equal(t, "", stdout)
		assert.NotEqual(t, "", stderr)
	}
	{
		// but selecting the stack should let you see it
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configA", "--stack", "testing-1")
		assert.Equal(t, "value1\n", stdout)
	}

	// Finally, check that the stack file contains what we expected.
	validate := func(k string, v string, cfg config.Map) {
		key, err := config.ParseKey("testing-config:config:" + k)
		assert.NoError(t, err)
		d, ok := cfg[key]
		assert.True(t, ok, "config key %v should be set", k)
		dv, err := d.Value(nil)
		assert.NoError(t, err)
		assert.Equal(t, v, dv)
	}

	testStack1, err := workspace.LoadProjectStack(filepath.Join(e.CWD, "Pulumi.testing-1.yaml"))
	assert.NoError(t, err)
	testStack2, err := workspace.LoadProjectStack(filepath.Join(e.CWD, "Pulumi.testing-2.yaml"))
	assert.NoError(t, err)

	assert.Equal(t, 2, len(testStack1.Config))
	assert.Equal(t, 2, len(testStack2.Config))

	validate("configA", "value1", testStack1.Config)
	validate("configC", "value3", testStack1.Config)

	validate("configB", "value2", testStack2.Config)
	validate("configD", "value4", testStack2.Config)

	e.RunCommand("pulumi", "stack", "rm", "--yes")
}

// Tests basic configuration from the perspective of a Pulumi program.
func TestConfigBasicNodeJS(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          filepath.Join("config_basic", "nodejs"),
		Dependencies: []string{"@pulumi/pulumi"},
		Quick:        true,
		Config: map[string]string{
			"aConfigValue": "this value is a value",
		},
		Secrets: map[string]string{
			"bEncryptedSecret": "this super secret is encrypted",
		},
	})
}

// Tests basic configuration from the perspective of a Pulumi program.
func TestConfigBasicPython(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:   filepath.Join("config_basic", "python"),
		Quick: true,
		Config: map[string]string{
			"aConfigValue": "this value is a Pythonic value",
		},
		Secrets: map[string]string{
			"bEncryptedSecret": "this super Pythonic secret is encrypted",
		},
	})
}
