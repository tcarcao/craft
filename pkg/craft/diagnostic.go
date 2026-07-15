package craft

import "github.com/tcarcao/craft/v2/internal/model"

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
