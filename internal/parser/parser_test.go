package parser

import (
	"testing"
)

func TestSimpleSystem(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(*testing.T, *Architecture)
	}{
		{
			name: "basic system",
			input: `system TestSystem {
                bounded context Test {
                }
            }`,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				if len(arch.Systems) != 1 {
					t.Errorf("Expected 1 system, got %d", len(arch.Systems))
				}
				if arch.Systems[0].Name != "TestSystem" {
					t.Errorf("Expected system name 'TestSystem', got %s", arch.Systems[0].Name)
				}
			},
		},
		{
			name: "system with components",
			input: `system TestSystem {
                bounded context Test {
                    component TestComp
                    service TestService
                    aggregate TestAggregate
                }
            }`,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				ctx := arch.Systems[0].Contexts[0]
				if len(ctx.Components) != 1 {
					t.Errorf("Expected 1 component, got %d", len(ctx.Components))
				}
				if len(ctx.Services) != 1 {
					t.Errorf("Expected 1 service, got %d", len(ctx.Services))
				}
				if len(ctx.Aggregates) != 1 {
					t.Errorf("Expected 1 aggregate, got %d", len(ctx.Aggregates))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()

			arch, err := parser.ParseString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, arch)
			}
		})
	}
}
