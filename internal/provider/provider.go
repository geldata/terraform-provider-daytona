package provider

import (
	"context"
	"os"

	"github.com/daytonaio/apiclient"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/geldata/terraform-provider-daytona/internal/datasources"
	"github.com/geldata/terraform-provider-daytona/internal/resources"
)

var _ provider.Provider = &DaytonaProvider{}

type DaytonaProvider struct {
	version string
}

type DaytonaProviderModel struct {
	Token          types.String `tfsdk:"token"`
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (p *DaytonaProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "daytona"
	resp.Version = p.version
}

func (p *DaytonaProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The Daytona provider is used to interact with Daytona resources through Terraform.",
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "JWT token for authenticating with the Daytona API. Can also be set via DAYTONA_TOKEN environment variable.",
			},
			"organization_id": schema.StringAttribute{
				Required:    true,
				Description: "Organization ID to use for requests.",
			},
		},
	}
}

func (p *DaytonaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data DaytonaProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := "https://app.daytona.io/api"

	token := os.Getenv("DAYTONA_TOKEN")
	if token == "" && !data.Token.IsNull() {
		token = data.Token.ValueString()
	}

	if token == "" {
		resp.Diagnostics.AddError(
			"Missing API Token",
			"The provider requires an API token to authenticate with Daytona. "+
				"Set it in the provider configuration or use the DAYTONA_TOKEN environment variable.",
		)
		return
	}

	organizationID := data.OrganizationID.ValueString()

	cfg := apiclient.NewConfiguration()
	cfg.Servers = []apiclient.ServerConfiguration{{
		URL: endpoint,
	}}
	cfg.DefaultHeader = map[string]string{
		"Authorization":             "Bearer " + token,
		"X-Daytona-Organization-ID": organizationID,
	}

	apiClient := apiclient.NewAPIClient(cfg)

	resp.DataSourceData = apiClient
	resp.ResourceData = apiClient
}

func (p *DaytonaProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewSnapshotResource,
	}
}

func (p *DaytonaProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewSnapshotDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &DaytonaProvider{
			version: version,
		}
	}
}
