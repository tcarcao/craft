package fmtconfig

import (
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	c := Defaults()
	if c.Indent != 4 {
		t.Errorf("Indent = %d, want 4", c.Indent)
	}
	if c.ContinuationIndent != 4 {
		t.Errorf("ContinuationIndent = %d, want 4", c.ContinuationIndent)
	}
	if c.Align.TrailingComment != ScopeBlock {
		t.Errorf("TrailingComment = %q, want %q", c.Align.TrailingComment, ScopeBlock)
	}
	if c.Align.OpAnnotation != ScopeBlock {
		t.Errorf("OpAnnotation = %q, want %q", c.Align.OpAnnotation, ScopeBlock)
	}
	if c.Align.OutlierRatio != 2.5 {
		t.Errorf("OutlierRatio = %v, want 2.5", c.Align.OutlierRatio)
	}
	if c.Align.OutlierMin != 40 {
		t.Errorf("OutlierMin = %d, want 40", c.Align.OutlierMin)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("Defaults().Validate() = %v, want nil", err)
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"negative indent", func(c *Config) { c.Indent = -1 }, "indent"},
		{"huge indent", func(c *Config) { c.Indent = 17 }, "indent"},
		{"unknown scope", func(c *Config) { c.Align.TrailingComment = "sometimes" }, "trailing_comment"},
		{"negative ratio", func(c *Config) { c.Align.OutlierRatio = -1 }, "outlier_ratio"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Defaults()
			tt.mut(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error mentioning %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate() = %q, want mention of %q", err, tt.want)
			}
		})
	}
}
