// internal/parser/parser_test.go
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
			},
		},
		{
			name: "component with technology",
			input: `system TestSystem {
                bounded context Test {
                    component TestComp using go
                }
            }`,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				comp := arch.Systems[0].Contexts[0].Components[0]
				if comp.Tech == nil || comp.Tech.Language != "go" {
					t.Errorf("Expected technology 'go', got %v", comp.Tech)
				}
			},
		},
		{
			name: "service with platform",
			input: `system TestSystem {
                bounded context Test {
                    service TestService using java on eks
                }
            }`,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				svc := arch.Systems[0].Contexts[0].Services[0]
				if svc.Tech == nil || svc.Tech.Language != "java" {
					t.Errorf("Expected language 'java', got %v", svc.Tech)
				}
				if svc.Platform != "eks" {
					t.Errorf("Expected platform 'eks', got %s", svc.Platform)
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

func TestFlows(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(*testing.T, *Architecture)
	}{
		{
			name: "simple flow",
			input: `
                system TestSystem {
                    bounded context Test {}
                }
                Test.Process() -> Other.Handle()
            `,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				if len(arch.Flows) != 1 {
					t.Errorf("Expected 1 flow, got %d", len(arch.Flows))
				}
			},
		},
		{
			name: "flow with arguments",
			input: `
                system TestSystem {
                    bounded context Test {}
                }
                Test.Process(arg1, arg2) -> Other.Handle()
            `,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				flow := arch.Flows[0]
				if len(flow.Args) != 2 {
					t.Errorf("Expected 2 arguments, got %d", len(flow.Args))
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
