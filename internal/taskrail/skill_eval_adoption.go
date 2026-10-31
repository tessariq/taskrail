package taskrail

import (
	"fmt"
	"slices"
)

func validateSkillEvalPinnedAdoption(input SkillEvalRunInput) error {
	stage := input.AdoptedBaselineStage
	if stage == nil {
		return nil
	}
	if len(input.AdoptedBaselineCases) == 0 {
		return fmt.Errorf("adopt baseline evidence: supplied stage requires explicitly enumerated baseline cases")
	}
	if stage.SchemaVersion != 1 || stage.Seal != skillEvalStageSeal(*stage) {
		return fmt.Errorf("adopt baseline evidence: supplied stage has an invalid seal or schema")
	}
	if stage.ArtifactAnchor == nil {
		return fmt.Errorf("adopt baseline evidence: supplied legacy stage has no sealed artifact root identity; use explicit legacy raw-copy adoption or execute a new stage")
	}
	if err := skillEvalValidateArtifactAnchor(input.ArtifactRoot, stage.ArtifactAnchor); err != nil {
		return fmt.Errorf("adopt baseline evidence: %w", err)
	}
	if stage.Report.SessionID != input.SessionID {
		return fmt.Errorf("adopt baseline evidence: supplied stage names a different raw session namespace")
	}
	for _, evaluation := range input.Registry {
		if !slices.Contains(input.AdoptedBaselineCases, evaluation.CaseID) {
			continue
		}
		prior := skillEvalPinnedCase(*stage, evaluation.CaseID)
		if prior == nil || prior.Skill != evaluation.Skill || prior.StorageMode != evaluation.StorageMode || prior.FixtureSHA256 != evaluation.FixtureSHA256 || !prior.BaselineRequired || prior.Baseline == nil {
			return fmt.Errorf("adopt baseline evidence: sealed baseline case or fixture does not match %q", evaluation.CaseID)
		}
		if prior.Baseline.SkillSHA256 != input.BaselineSkillSHA256[evaluation.Skill] || prior.Baseline.ExecutableSHA256 != input.BaselineExecutableSHA256 {
			return fmt.Errorf("adopt baseline evidence: sealed baseline skill or executable does not match %q", evaluation.CaseID)
		}
		if _, err := adoptSkillEvalBaselineArm(input, evaluation); err != nil {
			return err
		}
	}
	return nil
}

func skillEvalPinnedCase(stage SkillEvalStage, caseID string) *SkillEvalCaseReport {
	for index, item := range stage.Report.Cases {
		if item.CaseID == caseID {
			return &stage.Report.Cases[index]
		}
	}
	return nil
}
