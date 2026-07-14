package craft

import "github.com/tcarcao/craft/internal/model"

type (
	Diagnostic = model.Diagnostic
	Severity   = model.Severity
	Range      = model.Range
	Position   = model.Position
)

const (
	SeverityError   = model.SeverityError
	SeverityWarning = model.SeverityWarning
	SeverityInfo    = model.SeverityInfo
	SeverityHint    = model.SeverityHint
)
