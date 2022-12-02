package build

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/layer5io/meshery-adapter-library/adapter"
	"github.com/layer5io/meshery-consul/internal/config"

	"github.com/layer5io/meshkit/utils"
	"github.com/layer5io/meshkit/utils/kubernetes"
	"github.com/layer5io/meshkit/utils/manifests"
	smp "github.com/layer5io/service-mesh-performance/spec"
)

type Versions struct {
	AppVersion   string
	ChartVersion string
}

var DefaultGenerationMethod string
var LatestVersion string
var LatestAppVersion string
var WorkloadPath string
var MeshModelPath string
var CRDnames []string
var OverrideURL string
var AllVersions []Versions

var meshmodelmetadata = map[string]interface{}{
	"Primary Color":   "#D62783",
	"Secondary Color": "#ed74b4",
	"Shape":           "circle",
	"Logo URL":        "",
	"SVG_Color":       "",
	"SVG_White":       "<svg width=\"32\" height=\"32\" viewBox=\"0 0 32 32\" fill=\"none\" xmlns=\"http://www.w3.org/2000/svg\"><g clip-path=\"url(#a)\" fill=\"#fff\"><path fill-rule=\"evenodd\" clip-rule=\"evenodd\" d=\"M15.401 19.383c-1.873 0-3.33-1.503-3.33-3.436s1.457-3.436 3.33-3.436 3.33 1.503 3.33 3.436-1.457 3.436-3.33 3.436Zm6.504-1.825c-.832 0-1.56-.698-1.56-1.611 0-.86.675-1.61 1.56-1.61.832 0 1.561.697 1.561 1.61 0 .913-.728 1.61-1.56 1.61Zm5.62 1.503a1.516 1.516 0 0 1-1.874 1.126c-.832-.214-1.3-1.073-1.092-1.932a1.516 1.516 0 0 1 1.873-1.128c.779.214 1.3.966 1.144 1.825 0 0 0 .054-.052.107v.002Zm-1.095-4.082c-.833.214-1.665-.323-1.873-1.182a1.604 1.604 0 0 1 1.145-1.933c.832-.215 1.664.323 1.872 1.182.052.215.052.43 0 .644-.052.59-.468 1.128-1.144 1.289Zm5.515 3.865c-.157.859-.938 1.45-1.77 1.289a1.574 1.574 0 0 1-1.248-1.826c.155-.858.936-1.45 1.768-1.288.781.16 1.354.912 1.302 1.718-.052.053-.052.053-.052.107Zm-1.247-3.971c-.832.161-1.613-.43-1.769-1.288a1.575 1.575 0 0 1 1.25-1.826c.832-.161 1.612.43 1.768 1.289 0 .16.052.268 0 .429-.052.698-.572 1.289-1.249 1.396Zm-1.092 9.503c-.416.752-1.353 1.02-2.082.59-.728-.429-.988-1.395-.572-2.147.416-.752 1.353-1.02 2.081-.59.52.322.833.912.78 1.503-.051.215-.103.43-.207.644Zm-.573-14.603c-.728.43-1.665.16-2.08-.591-.417-.752-.157-1.718.572-2.148.728-.43 1.664-.16 2.08.591.157.322.209.59.209.913-.052.483-.312.966-.78 1.235\"/><path d=\"M15.453 32c-4.162 0-8.013-1.664-10.978-4.67A16.56 16.56 0 0 1 0 16c0-4.296 1.613-8.27 4.527-11.33C7.44 1.664 11.343 0 15.453 0c3.434 0 6.712 1.127 9.418 3.276l-1.924 2.576c-2.186-1.717-4.788-2.63-7.493-2.63-3.278 0-6.4 1.343-8.741 3.759-2.34 2.416-3.59 5.584-3.59 9.02 0 3.382 1.301 6.604 3.643 9.02 2.341 2.415 5.411 3.704 8.741 3.704 2.758 0 5.36-.913 7.492-2.63l1.874 2.576c-2.706 2.147-5.984 3.328-9.418 3.328l-.002.001Z\"/></g><defs><clipPath id=\"a\"><path fill=\"#fff\" d=\"M0 0h32v32H0z\"/></clipPath></defs></svg>",
}

var MeshModelConfig = adapter.MeshModelConfig{ //Move to build/config.go
	Category:    "Orchestration & Management",
	SubCategory: "Service Mesh",
	Metadata:    meshmodelmetadata,
}

// NewConfig creates the configuration for creating components
func NewConfig(version string) manifests.Config {
	return manifests.Config{
		Name:        smp.ServiceMesh_Type_name[int32(smp.ServiceMesh_CONSUL)],
		MeshVersion: version,
		CrdFilter: manifests.NewCueCrdFilter(manifests.ExtractorPaths{
			NamePath:    "spec.names.kind",
			IdPath:      "spec.names.kind",
			VersionPath: "spec.versions[0].name",
			GroupPath:   "spec.group",
			SpecPath:    "spec.versions[0].schema.openAPIV3Schema.properties.spec"}, false),
		ExtractCrds: func(manifest string) []string {
			crds := strings.Split(manifest, "---")
			return crds
		},
	}
}
func GetDefaultURL(crd string, version string) string {
	if OverrideURL != "" {
		return OverrideURL
	}
	return strings.Join([]string{fmt.Sprintf("https://raw.githubusercontent.com/hashicorp/consul-k8s/%s/control-plane/config/crd/bases", version), crd}, "/")
}
func init() {
	wd, _ := os.Getwd()
	WorkloadPath = filepath.Join(wd, "templates", "oam", "workloads")
	MeshModelPath = filepath.Join(wd, "templates", "meshmodel", "components")
	allVersions, _ := utils.GetLatestReleaseTagsSorted("hashicorp", "consul-k8s")
	if len(allVersions) == 0 {
		return
	}
	for i, v := range allVersions {
		if i == len(allVersions)-1 { //only get AppVersion of latest chart version
			//Executing the below function for all versions is redundant and takes time on startup, we only want to know the latest app version of latest version.
			av, err := kubernetes.HelmChartVersionToAppVersion("https://helm.releases.hashicorp.com", "consul", strings.TrimPrefix(v, "v"))
			if err != nil {
				log.Println("could not find app version for " + v + err.Error())
			}
			AllVersions = append(AllVersions, Versions{
				ChartVersion: v,
				AppVersion:   av,
			})
		} else {
			AllVersions = append(AllVersions, Versions{
				ChartVersion: v,
			})
		}
	}
	CRDnames, _ = config.GetFileNames("hashicorp", "consul-k8s", "control-plane/config/crd/bases/")
	LatestAppVersion = AllVersions[len(AllVersions)-1].AppVersion
	LatestVersion = AllVersions[len(AllVersions)-1].ChartVersion
	DefaultGenerationMethod = adapter.Manifests
}
