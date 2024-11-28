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
		{
			name: "complete system",
			input: `system OrderSystem {
                bounded context Orders {
                    aggregate Order
                    component OrderProcessor using go
                    service OrderService using php on eks
                    service QueueHandler using go on sqs
                    event OrderCreated
                    upstream to Payments as ohs
                }
            }`,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				ctx := arch.Systems[0].Contexts[0]
				if len(ctx.Services) != 2 {
					t.Errorf("Expected 2 services, got %d", len(ctx.Services))
				}
				if len(ctx.Components) != 1 {
					t.Errorf("Expected 1 component, got %d", len(ctx.Components))
				}
				if len(ctx.Relations) != 1 {
					t.Errorf("Expected 1 relation, got %d", len(ctx.Relations))
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

func TestComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		validate func(*testing.T, *Architecture)
	}{
		{
			name: "single line comments",
			input: `
                // System comment
                system TestSystem {
                    // Context comment
                    bounded context Test {
                        // Component comment
                        component TestComp using go // Inline comment
                        service TestService using java on eks // Service comment
                    }
                }
                // Flow comment
                Test.Process() -> Other.Handle() // Flow inline comment
            `,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				if len(arch.Systems) != 1 {
					t.Errorf("Expected 1 system, got %d", len(arch.Systems))
				}
				system := arch.Systems[0]
				if len(system.Contexts) != 1 {
					t.Errorf("Expected 1 context, got %d", len(system.Contexts))
				}
				if len(arch.Flows) != 1 {
					t.Errorf("Expected 1 flow, got %d", len(arch.Flows))
				}
			},
		},
		{
			name: "multi line comments",
			input: `
                /* This is a system
                   with multiple lines
                   of comments */
                system TestSystem {
                    /* Bounded context
                       with comments */
                    bounded context Test {
                        /* Multiple
                           components and
                           services */
                        component TestComp using go
                        service TestService using java on eks
                    }
                }
                /* Flow section
                   with routing */
                Test.Process() -> Other.Handle()
            `,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				if len(arch.Systems) != 1 {
					t.Errorf("Expected 1 system, got %d", len(arch.Systems))
				}
				system := arch.Systems[0]
				if len(system.Contexts) != 1 {
					t.Errorf("Expected 1 context, got %d", len(system.Contexts))
				}
				if len(arch.Flows) != 1 {
					t.Errorf("Expected 1 flow, got %d", len(arch.Flows))
				}
			},
		},
		{
			name: "mixed comments",
			input: `
                // Single line comment
                /* Multi-line
                   comment */
                system TestSystem { // Inline comment
                    /* Context
                       comment */
                    bounded context Test { // Another inline
                        // Component list
                        /* These are
                           our components */
                        component TestComp using go // With tech
                        service TestService using java on eks /* With platform */
                    }
                }
                // Flow comment
                /* Multiple
                   flows */
                Test.Process() -> Other.Handle() // Last comment
            `,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				if len(arch.Systems) != 1 {
					t.Errorf("Expected 1 system, got %d", len(arch.Systems))
				}
				system := arch.Systems[0]
				if len(system.Contexts) != 1 {
					t.Errorf("Expected 1 context, got %d", len(system.Contexts))
				}
				if len(arch.Flows) != 1 {
					t.Errorf("Expected 1 flow, got %d", len(arch.Flows))
				}
			},
		},
		{
			name: "nested comments error",
			input: `
                /* Outer comment
                   /* Nested comment */
                   Still outer */
                system TestSystem {
                    bounded context Test {}
                }
            `,
			wantErr: true,
		},
		{
			name: "unclosed multiline comment",
			input: `
                /* Unclosed comment
                system TestSystem {
                    bounded context Test {}
                }
            `,
			wantErr: true,
		},
		{
			name: "comments between tokens",
			input: `
                system /* comment */ TestSystem {
                    bounded /* with */ context /* more */ Test {
                        component /* inline */ TestComp /* tech */ using /* platform */ go
                    }
                }
            `,
			wantErr: false,
			validate: func(t *testing.T, arch *Architecture) {
				if len(arch.Systems) != 1 {
					t.Errorf("Expected 1 system, got %d", len(arch.Systems))
				}
				system := arch.Systems[0]
				if system.Name != "TestSystem" {
					t.Errorf("Expected system name 'TestSystem', got %q", system.Name)
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
