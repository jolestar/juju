// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package openstack

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"gopkg.in/goose.v1/errors"
	"gopkg.in/goose.v1/identity"
	"gopkg.in/goose.v1/neutron"
	"gopkg.in/goose.v1/nova"
	"gopkg.in/goose.v1/swift"

	"github.com/juju/juju/constraints"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/imagemetadata"
	"github.com/juju/juju/environs/instances"
	"github.com/juju/juju/environs/simplestreams"
	envstorage "github.com/juju/juju/environs/storage"
	envtesting "github.com/juju/juju/environs/testing"
	"github.com/juju/juju/instance"
	"github.com/juju/juju/network"
	"github.com/juju/juju/storage"
	"github.com/juju/juju/testing"
)

var (
	ShortAttempt   = &shortAttempt
	StorageAttempt = &storageAttempt
	CinderAttempt  = &cinderAttempt
)

// MetadataStorage returns a Storage instance which is used to store simplestreams metadata for tests.
func MetadataStorage(e environs.Environ) envstorage.Storage {
	env := e.(*Environ)
	ecfg := env.ecfg()
	container := "juju-dist-test"
	client, err := authClient(env.cloud, ecfg)
	if err != nil {
		panic(fmt.Errorf("cannot create %s container: %v", container, err))
	}
	metadataStorage := &openstackstorage{
		containerName: container,
		swift:         swift.New(client),
	}

	// Ensure the container exists.
	err = metadataStorage.makeContainer(container, swift.PublicRead)
	if err != nil {
		panic(fmt.Errorf("cannot create %s container: %v", container, err))
	}
	return metadataStorage
}

func InstanceAddress(publicIP string, addresses map[string][]nova.IPAddress) string {
	addr, _ := network.SelectPublicAddress(convertNovaAddresses(publicIP, addresses))
	return addr.Value
}

func InstanceServerDetail(inst instance.Instance) *nova.ServerDetail {
	return inst.(*openstackInstance).serverDetail
}

func InstanceFloatingIP(inst instance.Instance) *string {
	return inst.(*openstackInstance).floatingIP
}

var (
	NovaListAvailabilityZones   = &novaListAvailabilityZones
	AvailabilityZoneAllocations = &availabilityZoneAllocations
	NewOpenstackStorage         = &newOpenstackStorage
)

func NewCinderVolumeSource(s OpenstackStorage) storage.VolumeSource {
	return NewCinderVolumeSourceForModel(s, testing.ModelTag.Id())
}

func NewCinderVolumeSourceForModel(s OpenstackStorage, modelUUID string) storage.VolumeSource {
	const envName = "testenv"
	return &cinderVolumeSource{
		storageAdapter: s,
		envName:        envName,
		modelUUID:      modelUUID,
		namespace:      fakeNamespace{},
	}
}

type fakeNamespace struct {
	instance.Namespace
}

func (fakeNamespace) Value(s string) string {
	return "juju-" + s
}

// Include images for arches currently supported.  i386 is no longer
// supported, so it can be excluded.
var indexData = `
		{
		 "index": {
		  "com.ubuntu.cloud:released:openstack": {
		   "updated": "Wed, 01 May 2013 13:31:26 +0000",
		   "clouds": [
			{
			 "region": "{{.Region}}",
			 "endpoint": "{{.URL}}"
			}
		   ],
		   "cloudname": "test",
		   "datatype": "image-ids",
		   "format": "products:1.0",
		   "products": [
			"com.ubuntu.cloud:server:16.04:s390x",
			"com.ubuntu.cloud:server:16.04:amd64",
			"com.ubuntu.cloud:server:16.04:arm64",
			"com.ubuntu.cloud:server:16.04:ppc64el",
			"com.ubuntu.cloud:server:14.04:s390x",
			"com.ubuntu.cloud:server:14.04:amd64",
			"com.ubuntu.cloud:server:14.04:arm64",
			"com.ubuntu.cloud:server:14.04:ppc64el",
			"com.ubuntu.cloud:server:12.10:amd64",
			"com.ubuntu.cloud:server:13.04:amd64"
		   ],
		   "path": "image-metadata/products.json"
		  }
		 },
		 "updated": "Wed, 01 May 2013 13:31:26 +0000",
		 "format": "index:1.0"
		}
`

var imagesData = `
{
 "content_id": "com.ubuntu.cloud:released:openstack",
 "products": {
   "com.ubuntu.cloud:server:16.04:amd64": {
     "release": "trusty",
     "version": "16.04",
     "arch": "amd64",
     "versions": {
       "20121218": {
         "items": {
           "inst1": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "1"
           },
           "inst2": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "another-region",
             "id": "2"
           }
         },
         "pubname": "ubuntu-trusty-16.04-amd64-server-20121218",
         "label": "release"
       },
       "20121111": {
         "items": {
           "inst3": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "3"
           }
         },
         "pubname": "ubuntu-trusty-16.04-amd64-server-20121111",
         "label": "release"
       }
     }
   },
   "com.ubuntu.cloud:server:16.04:arm64": {
     "release": "xenial",
     "version": "16.04",
     "arch": "arm64",
     "versions": {
       "20121111": {
         "items": {
           "inst1604arm64": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "id-1604arm64"
           }
         },
         "pubname": "ubuntu-xenial-16.04-arm64-server-20121111",
         "label": "release"
       }
     }
   },
   "com.ubuntu.cloud:server:16.04:ppc64el": {
     "release": "xenial",
     "version": "16.04",
     "arch": "ppc64el",
     "versions": {
       "20121111": {
         "items": {
           "inst1604ppc64el": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "id-1604ppc64el"
           }
         },
         "pubname": "ubuntu-xenial-16.04-ppc64el-server-20121111",
         "label": "release"
       }
     }
   },
   "com.ubuntu.cloud:server:14.04:amd64": {
     "release": "trusty",
     "version": "14.04",
     "arch": "amd64",
     "versions": {
       "20121218": {
         "items": {
           "inst1": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "1"
           },
           "inst2": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "another-region",
             "id": "2"
           }
         },
         "pubname": "ubuntu-trusty-14.04-amd64-server-20121218",
         "label": "release"
       },
       "20121111": {
         "items": {
           "inst3": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "3"
           }
         },
         "pubname": "ubuntu-trusty-14.04-amd64-server-20121111",
         "label": "release"
       }
     }
   },
   "com.ubuntu.cloud:server:14.04:arm64": {
     "release": "trusty",
     "version": "14.04",
     "arch": "arm64",
     "versions": {
       "20121111": {
         "items": {
           "inst33": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "33"
           }
         },
         "pubname": "ubuntu-trusty-14.04-arm64-server-20121111",
         "label": "release"
       }
     }
   },
   "com.ubuntu.cloud:server:14.04:ppc64el": {
     "release": "trusty",
     "version": "14.04",
     "arch": "ppc64el",
     "versions": {
       "20121111": {
         "items": {
           "inst33": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "33"
           }
         },
         "pubname": "ubuntu-trusty-14.04-ppc64el-server-20121111",
         "label": "release"
       }
     }
   },
   "com.ubuntu.cloud:server:12.10:amd64": {
     "release": "quantal",
     "version": "12.10",
     "arch": "amd64",
     "versions": {
       "20121218": {
         "items": {
           "inst3": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "region-1",
             "id": "id-1"
           },
           "inst4": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "region-2",
             "id": "id-2"
           }
         },
         "pubname": "ubuntu-quantal-12.14-amd64-server-20121218",
         "label": "release"
       }
     }
   },
   "com.ubuntu.cloud:server:13.04:amd64": {
     "release": "raring",
     "version": "13.04",
     "arch": "amd64",
     "versions": {
       "20121218": {
         "items": {
           "inst5": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "id-y"
           },
           "inst6": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "another-region",
             "id": "id-z"
           }
         },
         "pubname": "ubuntu-raring-13.04-amd64-server-20121218",
         "label": "release"
       }
     }
   },
   "com.ubuntu.cloud:server:14.04:s390x": {
     "release": "trusty",
     "version": "14.04",
     "arch": "s390x",
     "versions": {
       "20121218": {
         "items": {
           "inst5": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "id-y"
           },
           "inst6": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "another-region",
             "id": "id-z"
           }
         },
         "pubname": "ubuntu-trusty-14.04-s390x-server-20121218",
         "label": "release"
       }
     }
   },
   "com.ubuntu.cloud:server:16.04:s390x": {
     "release": "xenial",
     "version": "16.04",
     "arch": "s390x",
     "versions": {
       "20121218": {
         "items": {
           "inst5": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "some-region",
             "id": "id-y"
           },
           "inst6": {
             "root_store": "ebs",
             "virt": "pv",
             "region": "another-region",
             "id": "id-z"
           }
         },
         "pubname": "ubuntu-xenial-16.04-s390x-server-20121218",
         "label": "release"
       }
     }
   }
 },
 "format": "products:1.0"
}
`

const productMetadatafile = "image-metadata/products.json"

func UseTestImageData(stor envstorage.Storage, cred *identity.Credentials) {
	// Put some image metadata files into the public storage.
	t := template.Must(template.New("").Parse(indexData))
	var metadata bytes.Buffer
	if err := t.Execute(&metadata, cred); err != nil {
		panic(fmt.Errorf("cannot generate index metdata: %v", err))
	}
	data := metadata.Bytes()
	stor.Put(simplestreams.UnsignedIndex("v1", 1), bytes.NewReader(data), int64(len(data)))
	stor.Put(
		productMetadatafile, strings.NewReader(imagesData), int64(len(imagesData)))

	envtesting.SignTestTools(stor)
}

func RemoveTestImageData(stor envstorage.Storage) {
	stor.RemoveAll()
}

// DiscardSecurityGroup cleans up a security group, it is not an error to
// delete something that doesn't exist.
func DiscardSecurityGroup(e environs.Environ, name string) error {
	env := e.(*Environ)
	neutronClient := env.neutron()
	groups, err := neutronClient.SecurityGroupByNameV2(name)
	if err != nil || len(groups) == 0 {
		if errors.IsNotFound(err) {
			// Group already deleted, done
			return nil
		}
	}
	for _, group := range groups {
		err = neutronClient.DeleteSecurityGroupV2(group.Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func FindInstanceSpec(
	e environs.Environ,
	series, arch, cons string,
	imageMetadata []*imagemetadata.ImageMetadata,
) (spec *instances.InstanceSpec, err error) {
	env := e.(*Environ)
	return findInstanceSpec(env, &instances.InstanceConstraint{
		Series:      series,
		Arches:      []string{arch},
		Region:      env.cloud.Region,
		Constraints: constraints.MustParse(cons),
	}, imageMetadata)
}

func GetSwiftURL(e environs.Environ) (string, error) {
	return e.(*Environ).clientUnlocked.MakeServiceURL("object-store", "", nil)
}

func SetUseFloatingIP(e environs.Environ, val bool) {
	env := e.(*Environ)
	env.ecfg().attrs["use-floating-ip"] = val
}

func SetUpGlobalGroup(e environs.Environ, name string, apiPort int) (neutron.SecurityGroupV2, error) {
	switching := e.(*Environ).firewaller.(*switchingFirewaller)
	if err := switching.initFirewaller(); err != nil {
		return neutron.SecurityGroupV2{}, err
	}
	return switching.fw.(*neutronFirewaller).setUpGlobalGroup(name, apiPort)
}

func EnsureGroup(e environs.Environ, name string, rules []neutron.RuleInfoV2) (neutron.SecurityGroupV2, error) {
	switching := e.(*Environ).firewaller.(*switchingFirewaller)
	if err := switching.initFirewaller(); err != nil {
		return neutron.SecurityGroupV2{}, err
	}
	return switching.fw.(*neutronFirewaller).ensureGroup(name, rules)
}

// ImageMetadataStorage returns a Storage object pointing where the goose
// infrastructure sets up its keystone entry for image metadata
func ImageMetadataStorage(e environs.Environ) envstorage.Storage {
	env := e.(*Environ)
	return &openstackstorage{
		containerName: "imagemetadata",
		swift:         swift.New(env.clientUnlocked),
	}
}

// CreateCustomStorage creates a swift container and returns the Storage object
// so you can put data into it.
func CreateCustomStorage(e environs.Environ, containerName string) envstorage.Storage {
	env := e.(*Environ)
	swiftClient := swift.New(env.clientUnlocked)
	if err := swiftClient.CreateContainer(containerName, swift.PublicRead); err != nil {
		panic(err)
	}
	return &openstackstorage{
		containerName: containerName,
		swift:         swiftClient,
	}
}

// BlankContainerStorage creates a Storage object with blank container name.
func BlankContainerStorage() envstorage.Storage {
	return &openstackstorage{}
}

// GetNeutronClient returns the neutron client for the current environs.
func GetNeutronClient(e environs.Environ) *neutron.Client {
	return e.(*Environ).neutron()
}

// GetNovaClient returns the nova client for the current environs.
func GetNovaClient(e environs.Environ) *nova.Client {
	return e.(*Environ).nova()
}

// ResolveNetwork exposes environ helper function resolveNetwork for testing
func ResolveNetwork(e environs.Environ, networkName string) (string, error) {
	return e.(*Environ).networking.ResolveNetwork(networkName)
}

var PortsToRuleInfo = rulesToRuleInfo
var SecGroupMatchesIngressRule = secGroupMatchesIngressRule

var MakeServiceURL = &makeServiceURL

var GetVolumeEndpointURL = getVolumeEndpointURL

func GetModelGroupNames(e environs.Environ) ([]string, error) {
	env := e.(*Environ)
	rawFirewaller := env.firewaller.(*switchingFirewaller).fw
	neutronFw, ok := rawFirewaller.(*neutronFirewaller)
	if !ok {
		return nil, fmt.Errorf("requires an env with a neutron firewaller")
	}
	groups, err := env.neutron().ListSecurityGroupsV2()
	if err != nil {
		return nil, err
	}
	modelPattern, err := regexp.Compile(neutronFw.jujuGroupRegexp())
	if err != nil {
		return nil, err
	}
	var results []string
	for _, group := range groups {
		if modelPattern.MatchString(group.Name) {
			results = append(results, group.Name)
		}
	}
	return results, nil
}

func GetFirewaller(e environs.Environ) Firewaller {
	env := e.(*Environ)
	return env.firewaller
}
