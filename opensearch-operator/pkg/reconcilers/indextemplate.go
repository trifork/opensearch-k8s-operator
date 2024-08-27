package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"time"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	"github.com/cisco-open/operator-tools/pkg/reconciler"
	"github.com/davecgh/go-spew/spew"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	opensearchIndexTemplateExists       = "index template already exists in OpenSearch; not modifying"
	opensearchIndexTemplateNameMismatch = "OpensearchIndexTemplateNameMismatch"
)

type IndexTemplateReconciler struct {
	client k8s.K8sClient
	ReconcilerOptions
	ctx      context.Context
	osClient *services.OsClusterClient
	recorder record.EventRecorder
	instance *opsterv1.OpensearchIndexTemplate
	cluster  *opsterv1.OpenSearchCluster
	logger   logr.Logger
}

func NewIndexTemplateReconciler(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	instance *opsterv1.OpensearchIndexTemplate,
	opts ...ReconcilerOption,
) *IndexTemplateReconciler {
	options := ReconcilerOptions{}
	options.apply(opts...)
	return &IndexTemplateReconciler{
		client:            k8s.NewK8sClient(client, ctx, reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "indextemplate"))),
		ReconcilerOptions: options,
		ctx:               ctx,
		recorder:          recorder,
		instance:          instance,
		logger:            log.FromContext(ctx).WithValues("reconciler", "indextemplate"),
	}
}

func (r *IndexTemplateReconciler) Reconcile() (retResult ctrl.Result, retErr error) {
	var reason string
	var templateName string

	defer func() {
		if !pointer.BoolDeref(r.updateStatus, true) {
			return
		}
		// When the reconciler is done, figure out what the state of the resource
		// is and set it in the state field accordingly.
		err := r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			instance := object.(*opsterv1.OpensearchIndexTemplate)
			instance.Status.Reason = reason
			if retErr != nil {
				instance.Status.State = opsterv1.OpensearchIndexTemplateError
			}
			if retResult.Requeue && retResult.RequeueAfter == 10*time.Second {
				instance.Status.State = opsterv1.OpensearchIndexTemplatePending
			}
			if retErr == nil && retResult.RequeueAfter == 30*time.Second {
				instance.Status.State = opsterv1.OpensearchIndexTemplateCreated
				instance.Status.IndexTemplateName = templateName
			}
			if reason == opensearchIndexTemplateExists {
				instance.Status.State = opsterv1.OpensearchIndexTemplateIgnored
			}
		})

		if err != nil {
			r.logger.Error(err, "failed to update status")
		}
	}()

	r.cluster, retErr = util.FetchOpensearchCluster(r.client, r.ctx, types.NamespacedName{
		Name:      r.instance.Spec.OpensearchRef.Name,
		Namespace: r.instance.Namespace,
	})
	if retErr != nil {
		reason = "error fetching opensearch cluster"
		r.logger.Error(retErr, "failed to fetch opensearch cluster")
		r.recorder.Event(r.instance, "Warning", opensearchError, reason)
		return
	}

	if r.cluster == nil {
		r.logger.Info("opensearch cluster does not exist, requeueing")
		reason = "waiting for opensearch cluster to exist"
		r.recorder.Event(r.instance, "Normal", opensearchPending, reason)
		retResult = ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}
		return
	}

	// Check cluster ref has not changed
	managedCluster := r.instance.Status.ManagedCluster
	if managedCluster != nil && *managedCluster != r.cluster.UID {
		reason = "cannot change the cluster an index template refers to"
		retErr = fmt.Errorf("%s", reason)
		r.recorder.Event(r.instance, "Warning", opensearchRefMismatch, reason)
		return ctrl.Result{
			Requeue: false,
		}, retErr
	}

	if pointer.BoolDeref(r.updateStatus, true) {
		retErr = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpensearchIndexTemplate).Status.ManagedCluster = &r.cluster.UID
		})
		if retErr != nil {
			reason = fmt.Sprintf("failed to update status: %s", retErr)
			r.recorder.Event(r.instance, "Warning", statusError, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: opensearchClusterRequeueAfter,
			}, retErr
		}
	}

	// Check cluster is ready
	if r.cluster.Status.Phase != opsterv1.PhaseRunning {
		r.logger.Info("opensearch cluster is not running, requeueing")
		reason = "waiting for opensearch cluster status to be running"
		r.recorder.Event(r.instance, "Normal", opensearchPending, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: opensearchClusterRequeueAfter,
		}, nil
	}

	r.osClient, retErr = util.CreateClientForCluster(r.client, r.ctx, r.cluster, r.osClientTransport)
	if retErr != nil {
		reason = "error creating opensearch client"
		r.recorder.Event(r.instance, "Warning", opensearchError, reason)
		r.logger.Error(retErr, "failed to create opensearch client")
		return
	}

	// If template name is not provided explicitly, use metadata.name by default
	templateName = r.instance.Name
	if r.instance.Spec.Name != "" {
		templateName = r.instance.Spec.Name
	}

	newTemplate := helpers.TranslateIndexTemplateToRequest(r.instance.Spec)

	existingTemplate, retErr := r.osClient.GetIndexTemplate(r.ctx, templateName)
	// If not exists, create
	if errors.Is(retErr, services.ErrNotFound) {
		r.logger.Info("Index template not found, creating new template", "templateName", templateName)
		retErr = r.osClient.PutIndexTemplate(r.ctx, templateName, newTemplate)
		if retErr != nil {
			reason = "failed to create index template"
			r.logger.Error(retErr, reason)
			r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, retErr
		}
		// Mark the ISM Policy as not pre-existing (created by the operator)
		retErr = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpensearchIndexTemplate).Status.IndexTemplateName = templateName
		})
		if retErr != nil {
			reason = "failed to update custom resource object"
			r.logger.Error(retErr, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, retErr
		}

		r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "index template successfully created in OpenSearch Cluster")
		r.logger.Info("Index template created successfully", "templateName", templateName)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}

	// If other error, report
	if retErr != nil {
		reason = "failed to get the index template from Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		r.logger.Info("Failed to get index template", "templateName", templateName)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, retErr
	}

	// If the index template exists in OpenSearch cluster and was not created by the operator, update the status and return
	if r.instance.Status.ExistingIndexTemplate == nil || *r.instance.Status.ExistingIndexTemplate {
		r.logger.Info("Index template already exists in OpenSearch cluster", "templateName", templateName)
		retErr = r.client.UdateObjectStatus(r.instance, func(object client.Object) {
			object.(*opsterv1.OpensearchIndexTemplate).Status.ExistingIndexTemplate = pointer.Bool(true)
		})
		if retErr != nil {
			reason = "failed to update custom resource object"
			r.logger.Error(retErr, reason)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: defaultRequeueAfter,
			}, retErr
		}
		reason = "the index template already exists in the OpenSearch cluster"
		r.logger.Error(errors.New(opensearchIndexTemplateExists), reason)
		r.recorder.Event(r.instance, "Warning", opensearchIndexTemplateExists, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}

	// Return if there are no changes
	r.logger.Info(spew.Sdump(existingTemplate))
	r.logger.Info(spew.Sdump(newTemplate))
	if r.instance.Spec.Name == existingTemplate.Name && cmp.Equal(*newTemplate, existingTemplate.IndexTemplate, cmpopts.EquateEmpty()) {
		r.logger.Info("Index template is in sync, no changes needed", "templateName", templateName)
		r.recorder.Event(r.instance, "Normal", opensearchAPIUnchanged, "index template is in sync")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, nil
	}

	r.logger.Info("Updating index template", "templateName", templateName)
	retErr = r.osClient.PutIndexTemplate(r.ctx, templateName, newTemplate)
	if retErr != nil {
		reason = "failed to update the index template with Opensearch API"
		r.logger.Error(retErr, reason)
		r.recorder.Event(r.instance, "Warning", opensearchAPIError, reason)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: defaultRequeueAfter,
		}, retErr
	}

	r.recorder.Event(r.instance, "Normal", opensearchAPIUpdated, "index template updated in OpenSearch")
	r.logger.Info("Index template updated successfully", "templateName", templateName)
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: defaultRequeueAfter,
	}, nil
}

func (r *IndexTemplateReconciler) Delete() error {
	// If we have never successfully reconciled we can just exit
	if r.instance.Status.ExistingIndexTemplate == nil {
		return nil
	}

	if *r.instance.Status.ExistingIndexTemplate {
		r.logger.Info("index template was pre-existing; not deleting")
		return nil
	}

	var err error

	r.cluster, err = util.FetchOpensearchCluster(r.client, r.ctx, types.NamespacedName{
		Name:      r.instance.Spec.OpensearchRef.Name,
		Namespace: r.instance.Namespace,
	})
	if err != nil {
		return err
	}

	if r.cluster == nil || !r.cluster.DeletionTimestamp.IsZero() {
		// If the opensearch cluster doesn't exist, we don't need to delete anything
		return nil
	}

	r.osClient, err = util.CreateClientForCluster(r.client, r.ctx, r.cluster, r.osClientTransport)
	if err != nil {
		return err
	}

	// If PolicyID not provided explicitly, use metadata.name by default
	templateName := r.instance.Name
	if r.instance.Spec.Name != "" {
		templateName = r.instance.Spec.Name
	}

	return services.DeleteIndexTemplate(r.ctx, r.osClient, templateName)
}
