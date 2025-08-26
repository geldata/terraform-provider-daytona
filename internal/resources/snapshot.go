package resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/daytonaio/apiclient"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &SnapshotResource{}
var _ resource.ResourceWithImportState = &SnapshotResource{}

func NewSnapshotResource() resource.Resource {
	return &SnapshotResource{}
}

type SnapshotResource struct {
	client *apiclient.APIClient
}

type SnapshotResourceModel struct {
	Id              types.String  `tfsdk:"id"`
	Name            types.String  `tfsdk:"name"`
	ImageName       types.String  `tfsdk:"image_name"`
	RemoteImageName types.String  `tfsdk:"remote_image_name"`
	OrganizationId  types.String  `tfsdk:"organization_id"`
	Size            types.Float32 `tfsdk:"size"`
	Cpu             types.Int32   `tfsdk:"cpu"`
	Gpu             types.Int32   `tfsdk:"gpu"`
	Memory          types.Int32   `tfsdk:"memory"`
	Disk            types.Int32   `tfsdk:"disk"`
	CreatedAt       types.String  `tfsdk:"created_at"`
	KeepRemotely    types.Bool    `tfsdk:"keep_remotely"`
}

func (r *SnapshotResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot"
}

func (r *SnapshotResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Daytona snapshot using local images and Daytona's container registry",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the snapshot",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the snapshot",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image_name": schema.StringAttribute{
				MarkdownDescription: "The local container image name for the snapshot",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"remote_image_name": schema.StringAttribute{
				MarkdownDescription: "The remote image name in Daytona's registry",
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
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(1),
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplace(),
				},
			},
			"gpu": schema.Int32Attribute{
				MarkdownDescription: "GPU units allocated to the resulting sandbox",
				Computed:            true,
			},
			"memory": schema.Int32Attribute{
				MarkdownDescription: "Memory allocated to the resulting sandbox in GB",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(1),
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplace(),
				},
			},
			"disk": schema.Int32Attribute{
				MarkdownDescription: "Disk space allocated to the resulting sandbox in GB",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(3),
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the snapshot",
				Computed:            true,
			},
			"keep_remotely": schema.BoolAttribute{
				MarkdownDescription: "Whether to keep the snapshot in Daytona when the Terraform resource is destroyed",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *SnapshotResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apiclient.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *apiclient.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *SnapshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *SnapshotResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	infos, warns, errors := r.createSnapshot(ctx, data)
	resp.Diagnostics.Append(infos...)
	resp.Diagnostics.Append(warns...)
	resp.Diagnostics.Append(errors...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SnapshotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *SnapshotResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	infoDiags, warnDiags, errDiags := r.readSnapshot(ctx, data)
	resp.Diagnostics.Append(infoDiags...)
	resp.Diagnostics.Append(warnDiags...)
	resp.Diagnostics.Append(errDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SnapshotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *SnapshotResourceModel
	var stateData SnapshotResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	shouldRecreate :=
		// recreate if image_name changes, except when importing (state has empty image_name)
		(!data.ImageName.Equal(stateData.ImageName) &&
			!(stateData.ImageName.ValueString() == "" && data.ImageName.ValueString() != "")) ||
			!data.Name.Equal(stateData.Name) ||
			!data.Cpu.Equal(stateData.Cpu) ||
			!data.Memory.Equal(stateData.Memory) ||
			!data.Disk.Equal(stateData.Disk)

	if shouldRecreate {
		if !data.KeepRemotely.ValueBool() {
			infos, warns, errors := r.deleteSnapshot(ctx, &stateData)
			resp.Diagnostics.Append(infos...)
			resp.Diagnostics.Append(warns...)
			resp.Diagnostics.Append(errors...)
			if resp.Diagnostics.HasError() {
				return
			}
		} else {
			tflog.Info(ctx, "Skipping old snapshot deletion during recreation due to keep_remotely=true", map[string]interface{}{
				"old_snapshot_id":   stateData.Id.ValueString(),
				"old_snapshot_name": stateData.Name.ValueString(),
			})
		}

		infos, warns, errors := r.createSnapshot(ctx, data)
		resp.Diagnostics.Append(infos...)
		resp.Diagnostics.Append(warns...)
		resp.Diagnostics.Append(errors...)
		if resp.Diagnostics.HasError() {
			return
		}

		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	} else {
		if data.Id.IsUnknown() {
			data.Id = stateData.Id
		}

		if data.Name.IsUnknown() {
			data.Name = stateData.Name
		}

		infos, warns, errors := r.readSnapshot(ctx, data)
		resp.Diagnostics.Append(infos...)
		resp.Diagnostics.Append(warns...)
		resp.Diagnostics.Append(errors...)
		if resp.Diagnostics.HasError() {
			return
		}

		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *SnapshotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *SnapshotResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.KeepRemotely.ValueBool() {
		tflog.Info(ctx, "Skipping snapshot deletion due to keep_remotely=true", map[string]interface{}{
			"snapshot_id":   data.Id.ValueString(),
			"snapshot_name": data.Name.ValueString(),
		})
		return
	}

	infos, warns, errors := r.deleteSnapshot(ctx, data)

	resp.Diagnostics.Append(infos...)
	resp.Diagnostics.Append(warns...)
	resp.Diagnostics.Append(errors...)
}

func (r *SnapshotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	snapshotID := req.ID

	snapshot, httpResp, err := r.client.SnapshotsAPI.GetSnapshot(ctx, snapshotID).Execute()
	if httpResp != nil && httpResp.Body != nil {
		defer httpResp.Body.Close()
	}
	if err != nil && httpResp != nil && httpResp.StatusCode == 404 {
		resp.Diagnostics.AddError(
			"Snapshot Not Found",
			fmt.Sprintf("Snapshot with ID/name %q not found", snapshotID),
		)
		return
	} else if err != nil {
		resp.Diagnostics.AddError(
			"Import Error",
			fmt.Sprintf("Unable to fetch snapshot %q: %v", snapshotID, err),
		)
		return
	}

	data := &SnapshotResourceModel{
		Id:              types.StringValue(snapshot.Id),
		Name:            types.StringValue(snapshot.Name),
		Cpu:             types.Int32Value(int32(snapshot.Cpu)),
		Gpu:             types.Int32Value(int32(snapshot.Gpu)),
		Memory:          types.Int32Value(int32(snapshot.Mem)),
		Disk:            types.Int32Value(int32(snapshot.Disk)),
		CreatedAt:       types.StringValue(snapshot.CreatedAt.Format("2006-01-02T15:04:05Z07:00")),
		OrganizationId:  types.StringPointerValue(snapshot.OrganizationId),
		Size:            types.Float32PointerValue(snapshot.Size.Get()),
		RemoteImageName: types.StringPointerValue(snapshot.ImageName),
		KeepRemotely:    types.BoolValue(false),

		// for now image_name is local only and we don't know it from the import...
		//
		// probably it would be better to add support for remote registry proxy for ECR
		// to fix this properly, but this works as a temporary hack as well
		ImageName: types.StringValue(""),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *SnapshotResource) createSnapshot(ctx context.Context, data *SnapshotResourceModel) (infos, warns, errs diag.Diagnostics) {
	warnings, errors := r.maybeCleanupExistingCreationAttempt(ctx, data.Name.ValueString())
	warns.Append(warnings...)
	errs.Append(errors...)
	if errs.HasError() {
		return
	}

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		errs.AddError("Docker Client Error", fmt.Sprintf("Unable to create Docker client: %v", err))
		return
	}
	defer dockerClient.Close()

	targetImage, warnings, errors := r.pushImageToRegistry(ctx, dockerClient, data.ImageName.ValueString())
	warns.Append(warnings...)
	errs.Append(errors...)
	if errs.HasError() {
		return
	}

	// we don't care too much about untagging. it's a garbage left behind, but not
	// a real error that prevents us from continuing
	defer func() {
		_, err = dockerClient.ImageRemove(ctx, targetImage, image.RemoveOptions{})
		if err != nil {
			warnings.AddWarning("Cleanup Warning", fmt.Sprintf("Failed to remove tagged image %s: %v", targetImage, err))
		}
	}()

	warnings, errors = r.registerSnapshot(ctx, data, targetImage)
	warns.Append(warnings...)
	errs.Append(errors...)
	if errs.HasError() {
		return
	}

	snapshot, warnings, errors := r.ensureSnapshotAvailable(ctx, data.Name.ValueString())
	warns.Append(warnings...)
	errs.Append(errors...)
	if errs.HasError() {
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
		data.OrganizationId = types.StringPointerValue(snapshot.OrganizationId)
	}
	if snapshot.ImageName != nil {
		data.RemoteImageName = types.StringPointerValue(snapshot.ImageName)
	}
	if snapshot.Size.IsSet() {
		data.Size = types.Float32PointerValue(snapshot.Size.Get())
	}

	return
}

func (r *SnapshotResource) maybeCleanupExistingCreationAttempt(ctx context.Context, snapshotName string) (warns, errors diag.Diagnostics) {
	existingSnapshot, httpResp, err := r.client.SnapshotsAPI.GetSnapshot(ctx, snapshotName).Execute()
	if httpResp != nil && httpResp.Body != nil {
		httpResp.Body.Close()
	}
	if err != nil && httpResp != nil && httpResp.StatusCode == 404 {
		return
	} else if err != nil {
		errors.AddError("Snapshot Check", fmt.Sprintf("Unable to check for if snapshot exists: %v", err))
		return
	}

	tflog.Info(ctx, "Found existing snapshot, deleting it", map[string]any{
		"snapshot_id":    existingSnapshot.Id,
		"snapshot_name":  existingSnapshot.Name,
		"snapshot_state": string(existingSnapshot.State),
	})

	_, err = r.client.SnapshotsAPI.RemoveSnapshot(ctx, existingSnapshot.Id).Execute()
	if err != nil {
		warns.AddWarning("Cleanup Warning", fmt.Sprintf("Failed to delete existing failed snapshot %q: %v", snapshotName, err))
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, httpResp, err := r.client.SnapshotsAPI.GetSnapshot(ctx, existingSnapshot.Id).Execute()
			if httpResp != nil && httpResp.Body != nil {
				httpResp.Body.Close()
			}
			if err != nil && httpResp != nil && httpResp.StatusCode == 404 {
				tflog.Info(ctx, "Snapshot successfully deleted")
				return
			}
			time.Sleep(time.Second)
		}
	}
}

func (r *SnapshotResource) pushImageToRegistry(ctx context.Context, dockerClient *client.Client, localImageName string) (targetImage string, warns, errors diag.Diagnostics) {
	tokenResponse, httpResp, err := r.client.DockerRegistryAPI.GetTransientPushAccess(ctx).Execute()
	if httpResp != nil && httpResp.Body != nil {
		httpResp.Body.Close()
	}
	if err != nil {
		errors.AddError("API Error", fmt.Sprintf("Unable to get push access token: %v", err))
		return
	}

	encodedAuth, err := json.Marshal(registry.AuthConfig{
		Username:      tokenResponse.Username,
		Password:      tokenResponse.Secret,
		ServerAddress: tokenResponse.RegistryUrl,
	})
	if err != nil {
		errors.AddError("Auth Error", fmt.Sprintf("Unable to encode docker auth config: %v", err))
		return
	}

	_, _, err = dockerClient.ImageInspectWithRaw(ctx, localImageName)
	if err != nil {
		errors.AddError("Image Not Found", fmt.Sprintf("Local image %q not found: %v", localImageName, err))
		return
	}

	localImageParts := strings.Split(localImageName, ":")
	localImageRepo := localImageParts[0]
	repoParts := strings.Split(localImageRepo, "/")
	imageName := repoParts[len(repoParts)-1]
	timestamp := time.Now().Format("20060102150405")
	targetImage = fmt.Sprintf("%s/%s/%s:%s", tokenResponse.RegistryUrl, tokenResponse.Project, imageName, timestamp)

	err = dockerClient.ImageTag(ctx, localImageName, targetImage)
	if err != nil {
		errors.AddError("Tag Error", fmt.Sprintf("Unable to tag image: %v", err))
		return
	}

	pushReader, err := dockerClient.ImagePush(ctx, targetImage, image.PushOptions{
		RegistryAuth: base64.URLEncoding.EncodeToString(encodedAuth),
	})
	if err != nil {
		errors.AddError("Push Error", fmt.Sprintf("Unable to push image: %v", err))
		return
	}
	defer pushReader.Close()

	_, err = io.Copy(io.Discard, pushReader)
	if err != nil {
		errors.AddError("Push Error", fmt.Sprintf("Error during image push: %v", err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			errors.AddError("Image Availability Error", fmt.Sprintf("Cancelled during waiting for image to become available: %v", ctx.Err()))
			return
		default:
			_, err = dockerClient.DistributionInspect(ctx, targetImage, base64.URLEncoding.EncodeToString(encodedAuth))
		}

		if err == nil {
			break
		}

		tflog.Info(ctx, "Waiting for the image to become available")
		time.Sleep(time.Second)
	}

	return
}

func (r *SnapshotResource) registerSnapshot(ctx context.Context, data *SnapshotResourceModel, targetImage string) (warns, errors diag.Diagnostics) {
	createRequest := apiclient.NewCreateSnapshot(data.Name.ValueString())
	createRequest.SetImageName(targetImage)

	if !data.Cpu.IsNull() {
		cpu := data.Cpu.ValueInt32()
		createRequest.Cpu = &cpu
	}

	if !data.Memory.IsNull() {
		memory := data.Memory.ValueInt32()
		createRequest.Memory = &memory
	}

	if !data.Disk.IsNull() {
		disk := data.Disk.ValueInt32()
		createRequest.Disk = &disk
	}

	_, resp, err := r.client.SnapshotsAPI.CreateSnapshot(ctx).CreateSnapshot(*createRequest).Execute()
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		errors.AddError("Client Error", fmt.Sprintf("Unable to create snapshot, got error: %v", err))
		return
	}

	return
}

func (r *SnapshotResource) ensureSnapshotAvailable(ctx context.Context, snapshotName string) (snapshot *apiclient.SnapshotDto, warns, errs diag.Diagnostics) {
	for {
		select {
		case <-ctx.Done():
			errs.AddError("Snapshot Availability Error", fmt.Sprintf("Cancelled during waiting for snapshot to become available: %v", ctx.Err()))
			return nil, warns, errs
		default:
			var resp *http.Response
			var err error
			snapshot, resp, err = r.client.SnapshotsAPI.GetSnapshot(ctx, snapshotName).Execute()
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			if err != nil {
				errs.AddError("Snapshot Availability Error", fmt.Sprintf("Unable to fetch snapshot: %v", err))
				return
			}

			switch snapshot.State {
			case apiclient.SNAPSHOTSTATE_ACTIVE:
				return
			case apiclient.SNAPSHOTSTATE_ERROR, apiclient.SNAPSHOTSTATE_BUILD_FAILED:
				if !snapshot.ErrorReason.IsSet() {
					errs.AddError("Snapshot Availability Error", "Snapshot processing failed with unknown reason")
				} else {
					errs.AddError("Snapshot Availability Error", fmt.Sprintf("Snapshot processing failed: %s", *snapshot.ErrorReason.Get()))
				}
				return
			}
		}

		tflog.Info(ctx, "Waiting for the snapshot to be processed")
		time.Sleep(time.Second)
	}
}

func (r *SnapshotResource) readSnapshot(ctx context.Context, data *SnapshotResourceModel) (infos, warns, errors diag.Diagnostics) {
	snapshot, resp, err := r.client.SnapshotsAPI.GetSnapshot(ctx, data.Id.ValueString()).Execute()
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil && resp != nil && resp.StatusCode == 404 {
		return
	} else if err != nil {
		errors.AddError("Client Error", fmt.Sprintf("Unable to read snapshot: %v", err))
		return
	}

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
		data.RemoteImageName = types.StringValue(*snapshot.ImageName)
	}

	if snapshot.Size.IsSet() {
		data.Size = types.Float32PointerValue(snapshot.Size.Get())
	}

	return
}

func (r *SnapshotResource) deleteSnapshot(ctx context.Context, data *SnapshotResourceModel) (infos, warns, errors diag.Diagnostics) {
	resp, err := r.client.SnapshotsAPI.RemoveSnapshot(ctx, data.Id.ValueString()).Execute()
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil && resp != nil && resp.StatusCode == 404 {
		return
	} else if err != nil {
		errors.AddError("Client Error", fmt.Sprintf("Unable to delete snapshot, got error: %v", err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			errors.AddError("Deletion Error", fmt.Sprintf("Cancelled while waiting for snapshot deletion: %v", ctx.Err()))
			return
		default:
			_, resp, err := r.client.SnapshotsAPI.GetSnapshot(ctx, data.Id.ValueString()).Execute()
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}
			if err != nil && resp != nil && resp.StatusCode == 404 {
				tflog.Info(ctx, "Snapshot successfully deleted")
				return
			} else if err != nil {
				errors.AddError("Deletion Verification Error", fmt.Sprintf("Unable to verify snapshot deletion: %v", err))
				return
			}

			tflog.Info(ctx, "Waiting for snapshot to be deleted")
			time.Sleep(time.Second)
		}
	}
}
