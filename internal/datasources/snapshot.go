package datasources

import (
	"context"
	"fmt"

	"github.com/daytonaio/apiclient"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &SnapshotDataSource{}

func NewSnapshotDataSource() datasource.DataSource {
	return &SnapshotDataSource{}
}

type SnapshotDataSource struct {
	client *apiclient.APIClient
}

type SnapshotDataSourceModel struct {
	Id             types.String  `tfsdk:"id"`
	Name           types.String  `tfsdk:"name"`
	ImageName      types.String  `tfsdk:"image_name"`
	Entrypoint     types.List    `tfsdk:"entrypoint"`
	OrganizationId types.String  `tfsdk:"organization_id"`
	Size           types.Float32 `tfsdk:"size"`
	Cpu            types.Int32   `tfsdk:"cpu"`
	Gpu            types.Int32   `tfsdk:"gpu"`
	Memory         types.Int32   `tfsdk:"memory"`
	Disk           types.Int32   `tfsdk:"disk"`
	CreatedAt      types.String  `tfsdk:"created_at"`
}

func (d *SnapshotDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot"
}

func (d *SnapshotDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches information about a Daytona snapshot",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the snapshot",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the snapshot",
				Required:            true,
			},
			"image_name": schema.StringAttribute{
				MarkdownDescription: "The container image name for the snapshot",
				Computed:            true,
			},
			"entrypoint": schema.ListAttribute{
				MarkdownDescription: "The entrypoint command for the snapshot",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The organization ID for the snapshot",
				Computed:            true,
			},
			"size": schema.Float32Attribute{
				MarkdownDescription: "The size of the snapshot in bytes",
				Computed:            true,
			},
			"cpu": schema.Int32Attribute{
				MarkdownDescription: "CPU cores allocated to the resulting sandbox",
				Computed:            true,
			},
			"gpu": schema.Int32Attribute{
				MarkdownDescription: "GPU units allocated to the resulting sandbox",
				Computed:            true,
			},
			"memory": schema.Int32Attribute{
				MarkdownDescription: "Memory allocated to the resulting sandbox in GB",
				Computed:            true,
			},
			"disk": schema.Int32Attribute{
				MarkdownDescription: "Disk space allocated to the resulting sandbox in GB",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the snapshot",
				Computed:            true,
			},
		},
	}
}

func (d *SnapshotDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apiclient.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *apiclient.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *SnapshotDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SnapshotDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	snapshot, httpResp, err := d.client.SnapshotsAPI.GetSnapshot(ctx, data.Name.ValueString()).Execute()
	if httpResp != nil && httpResp.Body != nil {
		httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to read snapshot, got error: %s", err),
		)
		return
	}

	data.Id = types.StringValue(snapshot.Id)
	data.Name = types.StringValue(snapshot.Name)
	data.Cpu = types.Int32Value(int32(snapshot.Cpu))
	data.Gpu = types.Int32Value(int32(snapshot.Gpu))
	data.Memory = types.Int32Value(int32(snapshot.Mem))
	data.Disk = types.Int32Value(int32(snapshot.Disk))
	data.CreatedAt = types.StringValue(snapshot.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))

	if snapshot.OrganizationId != nil {
		data.OrganizationId = types.StringValue(*snapshot.OrganizationId)
	}

	if snapshot.ImageName != nil {
		data.ImageName = types.StringValue(*snapshot.ImageName)
	}

	if snapshot.Size.IsSet() {
		data.Size = types.Float32PointerValue(snapshot.Size.Get())
	}

	if len(snapshot.Entrypoint) > 0 {
		entrypoint, diags := types.ListValueFrom(ctx, types.StringType, snapshot.Entrypoint)
		resp.Diagnostics.Append(diags...)
		data.Entrypoint = entrypoint
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
