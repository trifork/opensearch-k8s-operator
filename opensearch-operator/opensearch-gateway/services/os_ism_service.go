package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/opensearch-project/opensearch-go/opensearchutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var ErrNotFound = errors.New("policy not found")

// ShouldUpdateISMPolicy checks if the passed policy is same as existing or needs update
func ShouldUpdateISMPolicy(ctx context.Context, newPolicy, existingPolicy requests.ISMPolicy) (bool, error) {
	if cmp.Equal(newPolicy, existingPolicy, cmpopts.EquateEmpty()) {
		return false, nil
	}
	lg := log.FromContext(ctx).WithValues("os_service", "policy")
	lg.V(1).Info(fmt.Sprintf("existing policy: %+v", existingPolicy))
	lg.V(1).Info(fmt.Sprintf("new policy: %+v", newPolicy))
	lg.Info("policy exists and requires update")
	return true, nil
}

// GetPolicy fetches the passed policy
func GetPolicy(ctx context.Context, service *OsClusterClient, policyName string) (*responses.GetISMPolicyResponse, error) {
	resp, err := service.GetISMConfig(ctx, policyName)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, ErrNotFound
	}
	if resp.IsError() {
		return nil, fmt.Errorf("response from API is %s", resp.Status())
	}
	ismResponse := responses.GetISMPolicyResponse{}
	if resp != nil && resp.Body != nil {
		err := json.NewDecoder(resp.Body).Decode(&ismResponse)
		if err != nil {
			return nil, err
		}
		return &ismResponse, nil
	}
	return nil, fmt.Errorf("response is empty")
}

// CreateISMPolicy creates the passed policy
func CreateISMPolicy(ctx context.Context, service *OsClusterClient, ismpolicy requests.ISMPolicy, policyId string) error {
	spec := opensearchutil.NewJSONReader(ismpolicy)
	resp, err := service.PutISMConfig(ctx, policyId, spec)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("failed to create ism policy: %s", resp.String())
	}
	return nil
}

// UpdateISMPolicy updates the given policy
func UpdateISMPolicy(ctx context.Context, service *OsClusterClient, ismpolicy requests.ISMPolicy, seqno, primterm *int, policyId string) error {
	spec := opensearchutil.NewJSONReader(ismpolicy)
	resp, err := service.UpdateISMConfig(ctx, policyId, *seqno, *primterm, spec)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("failed to update ism policy: %s", resp.String())
	}
	return nil
}

// DeleteISMPolicy deletes the given policy
func DeleteISMPolicy(ctx context.Context, service *OsClusterClient, policyName string) error {
	resp, err := service.DeleteISMConfig(ctx, policyName)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("failed to delete ism policy: %s", resp.String())
	}
	return nil
}

// IndexInfo represents the response from OpenSearch for indices
type IndexInfo struct {
    Index string `json:"index"`
}

// GetIndices retrieves indices matching the given pattern from OpenSearch
func GetIndices(ctx context.Context, service *OsClusterClient, pattern string) ([]string, error) {
    resp, err := service.GetIndices(ctx, pattern)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.IsError() {
        return nil, fmt.Errorf("failed to get indices: %s", resp.String())
    }

    var indices []IndexInfo
    if resp != nil && resp.Body != nil {
        err := json.NewDecoder(resp.Body).Decode(&indices)
        if err != nil {
            return nil, err
        }
        indexNames := make([]string, 0, len(indices))
        for _, idx := range indices {
            indexNames = append(indexNames, idx.Index)
        }
        return indexNames, nil
    }
    return nil, fmt.Errorf("response is empty")
}

// AddISMPolicyRequest is the request body for adding an ISM policy to an index
type AddISMPolicyRequest struct {
    PolicyID string `json:"policy_id"`
}

// AddPolicyToIndex applies an ISM policy to the specified index
func AddPolicyToIndex(ctx context.Context, service *OsClusterClient, indexName string, policyId string) error {
    body := opensearchutil.NewJSONReader(AddISMPolicyRequest{PolicyID: policyId})
	
	resp, err := service.AddPolicyToIndex(ctx, indexName, body)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.IsError() {
        return fmt.Errorf("failed to add policy to index: %s", resp.String())
    }

    return nil
}