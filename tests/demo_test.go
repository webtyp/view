package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/ddl"
	"webtyp.com/model"
	"webtyp.com/view"
)

type mockDemoModule struct{}

func (m mockDemoModule) ModelName() string                   { return "demo_mod" }
func (m mockDemoModule) Definition() model.ModuleDefinition { return model.ModuleDefinition{Name: m.ModelName()} }
func (m mockDemoModule) MountOperations(reg crud.Registry) {}

func TestDemo_ValidateNamesFirstMissingField(t *testing.T) {
	tests := []struct {
		name    string
		demo    view.Demo
		wantErr error
	}{
		{
			name:    "zero Demo",
			demo:    view.Demo{},
			wantErr: view.ErrDemoNoLabel,
		},
		{
			name: "Label only",
			demo: view.Demo{
				Label: "Patient Directory",
			},
			wantErr: view.ErrDemoNoTimezone,
		},
		{
			name: "Label + Timezone",
			demo: view.Demo{
				Label:    "Patient Directory",
				Timezone: "America/Santiago",
			},
			wantErr: view.ErrDemoNoModules,
		},
		{
			name: "Label + Timezone + Modules",
			demo: view.Demo{
				Label:    "Patient Directory",
				Timezone: "America/Santiago",
				Modules:  []crud.Module{mockDemoModule{}},
			},
			wantErr: view.ErrDemoNoScreens,
		},
		{
			name: "complete Demo",
			demo: view.Demo{
				Label:      "Patient Directory",
				Timezone:   "America/Santiago",
				ActingAs:   "u1",
				Modules:    []crud.Module{mockDemoModule{}},
				Migrations: []ddl.Step{},
				Screens: func(d view.BrowserDeps) []view.Screen {
					return nil
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.demo.Validate()
			if err != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
