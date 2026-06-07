package httptransport

import "github.com/alryzden/ProtoRadar/internal/edition"

type editionResponse struct {
	Edition      string                `json:"edition"`
	Version      string                `json:"version"`
	Commit       string                `json:"commit"`
	BuildDate    string                `json:"build_date"`
	Capabilities []capabilityStatusDTO `json:"capabilities"`
}

type capabilityStatusDTO struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func editionDTO(model edition.Edition) editionResponse {
	capabilities := make([]capabilityStatusDTO, 0, len(model.Capabilities))
	for _, status := range model.Capabilities {
		capabilities = append(capabilities, capabilityStatusDTO{
			Name:    status.Capability.String(),
			Enabled: status.Enabled,
		})
	}
	return editionResponse{
		Edition:      model.Name,
		Version:      model.Version.Version,
		Commit:       model.Version.Commit,
		BuildDate:    model.Version.BuildDate,
		Capabilities: capabilities,
	}
}
