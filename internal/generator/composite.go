package generator

import (
	"context"
	"log/slog"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

// SubGenerator is the interface for individual test case generators (rule or AI).
type SubGenerator interface {
	Generate(ctx context.Context, endpoint models.Endpoint) ([]models.TestCase, error)
}

// CompositeGenerator combines rule-based generation with optional AI-powered generation.
type CompositeGenerator struct {
	ruleGen SubGenerator
	aiGen   SubGenerator
	logger  *slog.Logger
}

// NewCompositeGenerator creates a new CompositeGenerator.
// aiGen can be nil if AI generation is disabled or unconfigured.
func NewCompositeGenerator(ruleGen SubGenerator, aiGen SubGenerator, logger *slog.Logger) *CompositeGenerator {
	if logger == nil {
		logger = slog.Default()
	}
	return &CompositeGenerator{
		ruleGen: ruleGen,
		aiGen:   aiGen,
		logger:  logger,
	}
}

// Generate executes the rule-based generator first, then optionally runs the AI generator,
// merging both sets of test cases into a single slice.
func (c *CompositeGenerator) Generate(ctx context.Context, endpoint models.Endpoint) ([]models.TestCase, error) {
	// 1. Always run rule-based generator
	cases, err := c.ruleGen.Generate(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	// 2. Run AI generator if configured
	if c.aiGen != nil {
		aiCases, err := c.aiGen.Generate(ctx, endpoint)
		if err != nil {
			c.logger.Warn("composite generator: AI generation failed, falling back to rule-based test cases only", 
				"error", err,
				"endpoint_id", endpoint.ID,
				"method", endpoint.Method,
				"path", endpoint.Path,
			)
		} else if len(aiCases) > 0 {
			cases = append(cases, aiCases...)
		}
	}

	return cases, nil
}
