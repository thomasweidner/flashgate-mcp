package main

import (
	"bytes"
	"errors"
	"path/filepath"
)

// This is a NEW human-governed one-shot assertion for one exact empty draft.
// The original plan is immutable evidence, never renewed authorization to create.
type publicationContinuationAuthorization struct {
	Schema           string `json:"schema"`
	Action           string `json:"action"`
	Repository       string `json:"repository"`
	ReleaseID        int64  `json:"releaseId"`
	PreState         string `json:"preState"`
	OriginalApproval string `json:"originalApproval"`
	PlanSHA256       string `json:"planSha256"`
}

type publicationContinuationPlan struct {
	Schema              string                               `json:"schema"`
	Authorization       publicationContinuationAuthorization `json:"authorization"`
	AuthorizationSHA256 string                               `json:"authorizationSha256"`
}

func publicationReadFile(name string) ([]byte, error) {
	root, err := supplyOpen(filepath.Dir(name))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return supplyRead(root, filepath.Base(name))
}

func readPublicationRequest(name, anchor, manifest string) (publicationAuthorization, *publicationContinuationAuthorization, *publicationPlan, error) {
	if manifest == "" {
		a, err := readPublicationAuthorization(name, anchor)
		return a, nil, nil, err
	}
	var continuation publicationContinuationAuthorization
	var plan publicationPlan
	data, err := publicationReadFile(name)
	if err != nil || !hashPattern.MatchString(anchor) || supplyDigest(data) != anchor {
		return plan.Authorization, nil, nil, errors.New("continuation authorization digest mismatch")
	}
	if err := strictCandidateBytes(data, &continuation); err != nil {
		return plan.Authorization, nil, nil, err
	}
	if continuation.Schema != "flashgate-publication-continuation-authorization/v1" ||
		continuation.Action != "COMPLETE_EMPTY_DRAFT_UPLOAD_READBACK_PUBLISH_PRERELEASE" ||
		continuation.Repository != publicationRepository || continuation.ReleaseID < 1 ||
		continuation.PreState != "EXACT_EMPTY_DRAFT" || !hashPattern.MatchString(continuation.PlanSHA256) ||
		!publicationApprovalPattern.MatchString(continuation.OriginalApproval) {
		return plan.Authorization, nil, nil, errors.New("invalid empty-draft continuation authorization")
	}
	leaks := leakReport{}
	scanBytes("continuation approval", []byte(continuation.OriginalApproval), nil, &leaks)
	if len(leaks.Findings) != 0 {
		return plan.Authorization, nil, nil, errors.New("unsafe continuation approval reference")
	}
	original, err := publicationReadFile(manifest)
	if err != nil || supplyDigest(original) != continuation.PlanSHA256 {
		return plan.Authorization, nil, nil, errors.New("original publication manifest digest mismatch")
	}
	if err := strictCandidateBytes(original, &plan); err != nil {
		return plan.Authorization, nil, nil, err
	}
	canonical, err := supplyJSON(plan)
	if err != nil || !bytes.Equal(canonical, original) || plan.Schema != "flashgate-publication-manifest/v1" ||
		validatePublicationAuthorization(plan.Authorization) != nil || !hashPattern.MatchString(plan.AuthorizationSHA256) ||
		continuation.OriginalApproval == plan.Authorization.OriginalApproval {
		return plan.Authorization, nil, nil, errors.New("original plan invalid or consumed approval reused")
	}
	return plan.Authorization, &continuation, &plan, nil
}

func preparePublicationRequest(root, dir string, a publicationAuthorization, anchor string, continuation *publicationContinuationAuthorization, original *publicationPlan) (publicationPlan, error) {
	if continuation == nil {
		return preparePublication(root, dir, a, anchor)
	}
	if original == nil {
		return publicationPlan{}, errors.New("bound original plan required")
	}
	// Reuse the complete source/evidence verifier, including all five anchors.
	plan, err := preparePublication(root, dir, a, original.AuthorizationSHA256)
	if err != nil {
		return plan, err
	}
	data, err := supplyJSON(plan)
	if err != nil || supplyDigest(data) != continuation.PlanSHA256 {
		return plan, errors.New("original publication plan changed during continuation preflight")
	}
	return plan, nil
}
