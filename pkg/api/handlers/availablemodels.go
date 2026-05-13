package handlers

import (
	"strings"

	"github.com/obot-platform/obot/pkg/api/handlers/providers"

	openai "github.com/gptscript-ai/chat-completion-client"
	"github.com/obot-platform/obot/apiclient/types"
	"github.com/obot-platform/obot/pkg/api"
	"github.com/obot-platform/obot/pkg/gateway/server/dispatcher"
	"github.com/obot-platform/obot/pkg/license"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/system"
	"k8s.io/apimachinery/pkg/fields"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type AvailableModelsHandler struct {
	dispatcher      *dispatcher.Dispatcher
	licenseProvider *license.KeygenProvider
}

func NewAvailableModelsHandler(dispatcher *dispatcher.Dispatcher, licenseProvider *license.KeygenProvider) *AvailableModelsHandler {
	return &AvailableModelsHandler{
		dispatcher:      dispatcher,
		licenseProvider: licenseProvider,
	}
}

func (a *AvailableModelsHandler) List(req api.Context) error {
	var modelProviderReferences v1.ToolReferenceList
	if err := req.List(&modelProviderReferences, &kclient.ListOptions{
		Namespace: req.Namespace(),
		FieldSelector: fields.SelectorFromSet(map[string]string{
			"spec.type": string(v1.ToolReferenceTypeModelProvider),
		}),
	}); err != nil {
		return err
	}

	var oModels openai.ModelsList
	for _, modelProvider := range modelProviderReferences.Items {
		convertedModelProvider, err := providers.ConvertModelProviderToolRef(modelProvider, nil, a.licenseProvider)
		if err != nil {
			log.Warnf("failed to convert model provider %q: %v", modelProvider.Name, err)
			continue
		}
		if !convertedModelProvider.Configured || modelProvider.Name == system.ModelProviderTool {
			continue
		}

		m, err := a.dispatcher.ModelsForProvider(req.Context(), req.GPTClient, modelProvider.Namespace, modelProvider.Name)
		if err != nil {
			return err
		}

		for _, model := range m.Models {
			if model.Metadata == nil {
				model.Metadata = make(map[string]string)
			}
			model.Metadata["model-provider"] = modelProvider.Name
			oModels.Models = append(oModels.Models, model)
		}
	}

	return req.Write(oModels)
}

func (a *AvailableModelsHandler) ListForModelProvider(req api.Context) error {
	modelProviderID := req.PathValue("model_provider_id")
	var modelProviderReference v1.ToolReference
	if err := req.Get(&modelProviderReference, modelProviderID); err != nil {
		return err
	}

	if modelProviderReference.Spec.Type != v1.ToolReferenceTypeModelProvider {
		return types.NewErrBadRequest("%s is not a model provider", modelProviderReference.Name)
	}

	modelProvider, err := providers.ConvertModelProviderToolRef(modelProviderReference, nil, a.licenseProvider)
	if err != nil {
		return err
	}

	if !modelProvider.Configured {
		return types.NewErrBadRequest("model provider %s is not configured, missing configuration parameters: %s", modelProviderReference.Name, strings.Join(modelProvider.MissingConfigurationParameters, ", "))
	}

	oModels, err := a.dispatcher.ModelsForProvider(req.Context(), req.GPTClient, modelProviderReference.Namespace, modelProviderReference.Name)
	if err != nil {
		return err
	}

	return req.Write(oModels)
}
