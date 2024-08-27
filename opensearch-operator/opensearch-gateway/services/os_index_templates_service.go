package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/requests"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/responses"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"github.com/opensearch-project/opensearch-go/opensearchutil"
)

func (client *OsClusterClient) DeleteIndexTemplate(ctx context.Context, name string) error {
	req := opensearchapi.IndicesDeleteIndexTemplateRequest{
		Name: name,
	}
	resp, err := req.Do(context.Background(), client.client)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return ErrNotFound
	}

	if resp.IsError() {
		return fmt.Errorf("response from API is %s", resp.Status())
	}

	return nil
}

// DeleteIndexTemplate deletes a previously created index template
func DeleteIndexTemplate(ctx context.Context, service *OsClusterClient, indexTemplateName string) error {
	path := IndexTemplatePath(indexTemplateName)
	resp, err := doHTTPDelete(ctx, service.client, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("response from API is %s", resp.Status())
	}
	return nil
}

func (client *OsClusterClient) GetIndexTemplate(ctx context.Context, name string) (*responses.IndexTemplate, error) {
	req := opensearchapi.IndicesGetIndexTemplateRequest{
		Name: []string{name},
	}
	resp, err := req.Do(context.Background(), client.client)
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

	response := &responses.GetIndexTemplatesResponse{}
	err = json.NewDecoder(resp.Body).Decode(response)
	if err != nil {
		return nil, err
	}

	if len(response.IndexTemplates) != 1 {
		return nil, fmt.Errorf("found %d index templates that fit the name '%s', expected 1", len(response.IndexTemplates), name)
	}
	return &response.IndexTemplates[0], nil
}

func (client *OsClusterClient) PutIndexTemplate(ctx context.Context, name string, indexTemplate *requests.IndexTemplate) error {
	req := opensearchapi.IndicesPutIndexTemplateRequest{
		Name: name,
		Body: opensearchutil.NewJSONReader(indexTemplate),
	}
	resp, err := req.Do(context.Background(), client.client)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("failed to create index template: %s", resp.String())
	}
	return nil
}

// IndexTemplatePath returns a strings.Builder pointing to /_index_template/<templateName>
func IndexTemplatePath(templateName string) strings.Builder {
	var path strings.Builder
	path.Grow(len("/_index_template/") + len(templateName))
	path.WriteString("/_index_template/")
	path.WriteString(templateName)
	return path
}
