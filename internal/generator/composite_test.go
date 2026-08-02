package generator

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

type mockSubGenerator struct {
	cases []models.TestCase
	err   error
}

func (m *mockSubGenerator) Generate(ctx context.Context, endpoint models.Endpoint) ([]models.TestCase, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.cases, nil
}

func TestCompositeGeneratorRulesOnly(t *testing.T) {
	ruleCases := []models.TestCase{
		{Category: models.CategoryPositive, ExpectedStatus: 200},
		{Category: models.CategoryNegative, ExpectedStatus: 400},
	}
	ruleGen := &mockSubGenerator{cases: ruleCases}

	comp := NewCompositeGenerator(ruleGen, nil, nil)
	result, err := comp.Generate(context.Background(), models.Endpoint{ID: uuid.New()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(result))
	}
}

func TestCompositeGeneratorWithAI(t *testing.T) {
	ruleCases := []models.TestCase{
		{Category: models.CategoryPositive, ExpectedStatus: 200},
	}
	aiCases := []models.TestCase{
		{Category: models.CategoryAIEnhanced, ExpectedStatus: 422},
	}

	ruleGen := &mockSubGenerator{cases: ruleCases}
	aiGen := &mockSubGenerator{cases: aiCases}

	comp := NewCompositeGenerator(ruleGen, aiGen, nil)
	result, err := comp.Generate(context.Background(), models.Endpoint{ID: uuid.New()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(result))
	}
	if result[1].Category != models.CategoryAIEnhanced {
		t.Errorf("expected 2nd case to be ai_enhanced, got %s", result[1].Category)
	}
}

func TestCompositeGeneratorAIFailFallback(t *testing.T) {
	ruleCases := []models.TestCase{
		{Category: models.CategoryPositive, ExpectedStatus: 200},
	}

	ruleGen := &mockSubGenerator{cases: ruleCases}
	aiGen := &mockSubGenerator{err: errors.New("bedrock timeout")}

	comp := NewCompositeGenerator(ruleGen, aiGen, nil)
	result, err := comp.Generate(context.Background(), models.Endpoint{ID: uuid.New()})
	if err != nil {
		t.Fatalf("expected nil error on AI failure, got %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 case from rule-based fallback, got %d", len(result))
	}
}
