/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package machineset

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/api/core/v1beta2/index"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/cluster-api/controllers/remote"
	machinecontroller "sigs.k8s.io/cluster-api/internal/controllers/machine"
	"sigs.k8s.io/cluster-api/internal/test/envtest"
)

const (
	timeout         = time.Second * 30
	testClusterName = "test-cluster"
)

var (
	env        *envtest.Environment
	ctx        = ctrl.SetupSignalHandler()
	fakeScheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(fakeScheme)
	_ = clusterv1.AddToScheme(fakeScheme)
	_ = apiextensionsv1.AddToScheme(fakeScheme)
}

func TestMain(m *testing.M) {
	setupIndexes := func(ctx context.Context, mgr ctrl.Manager) {
		if err := index.AddDefaultIndexes(ctx, mgr); err != nil {
			panic(fmt.Sprintf("unable to setup index: %v", err))
		}
	}

	setupReconcilers := func(ctx context.Context, mgr ctrl.Manager) {
		clusterCache, err := clustercache.SetupWithManager(ctx, mgr, clustercache.Options{
			SecretClient: mgr.GetClient(),
			Cache: clustercache.CacheOptions{
				Indexes: []clustercache.CacheOptionsIndex{clustercache.NodeProviderIDIndex},
			},
			Client: clustercache.ClientOptions{
				UserAgent: remote.DefaultClusterAPIUserAgent("test-controller-manager"),
				Cache: clustercache.ClientCacheOptions{
					DisableFor: []client.Object{
						// Don't cache ConfigMaps & Secrets.
						&corev1.ConfigMap{},
						&corev1.Secret{},
					},
				},
			},
		}, controller.Options{MaxConcurrentReconciles: 10})
		if err != nil {
			panic(fmt.Sprintf("Failed to create ClusterCache: %v", err))
		}
		go func() {
			<-ctx.Done()
			clusterCache.(interface{ Shutdown() }).Shutdown()
		}()

		if err := (&Reconciler{
			Client:       mgr.GetClient(),
			APIReader:    mgr.GetAPIReader(),
			ClusterCache: clusterCache,
		}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: 1}); err != nil {
			panic(fmt.Sprintf("Failed to start MMachineSetReconciler: %v", err))
		}
		if err := (&machinecontroller.Reconciler{
			Client:                      mgr.GetClient(),
			APIReader:                   mgr.GetAPIReader(),
			ClusterCache:                clusterCache,
			RemoteConditionsGracePeriod: 5 * time.Minute,
		}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: 1}); err != nil {
			panic(fmt.Sprintf("Failed to start MachineReconciler: %v", err))
		}
	}

	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultEventuallyTimeout(timeout)

	req, _ := labels.NewRequirement(clusterv1.ClusterNameLabel, selection.Exists, nil)
	clusterSecretCacheSelector := labels.NewSelector().Add(*req)

	os.Exit(envtest.Run(ctx, envtest.RunInput{
		M: m,
		ManagerCacheOptions: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				// Only cache Secrets with the cluster name label.
				// This is similar to the real world.
				&corev1.Secret{}: {
					Label: clusterSecretCacheSelector,
				},
			},
		},
		ManagerUncachedObjs: []client.Object{
			&corev1.ConfigMap{},
			&corev1.Secret{},
		},
		SetupEnv:         func(e *envtest.Environment) { env = e },
		SetupIndexes:     setupIndexes,
		SetupReconcilers: setupReconcilers,
	}))
}

func fakeBootstrapRefDataSecretCreated(ref clusterv1.ContractVersionedObjectReference, namespace string, base map[string]interface{}, g *WithT) {
	bref := (&unstructured.Unstructured{Object: base}).DeepCopy()
	g.Eventually(func() error {
		return env.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, bref)
	}).Should(Succeed())

	bdataSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: ref.Name,
			Namespace:    namespace,
		},
		StringData: map[string]string{
			"value": "data",
		},
	}
	g.Expect(env.Create(ctx, bdataSecret)).To(Succeed())

	brefPatch := client.MergeFrom(bref.DeepCopy())
	g.Expect(unstructured.SetNestedField(bref.Object, true, "status", "initialization", "dataSecretCreated")).To(Succeed())
	g.Expect(unstructured.SetNestedField(bref.Object, bdataSecret.Name, "status", "dataSecretName")).To(Succeed())
	g.Expect(env.Status().Patch(ctx, bref, brefPatch)).To(Succeed())
}

func fakeInfrastructureRefProvisioned(ref clusterv1.ContractVersionedObjectReference, namespace string, base map[string]interface{}, g *WithT) string {
	iref := (&unstructured.Unstructured{Object: base}).DeepCopy()
	g.Eventually(func() error {
		return env.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, iref)
	}).Should(Succeed())

	irefPatch := client.MergeFrom(iref.DeepCopy())
	providerID := fmt.Sprintf("test:////%v", uuid.NewUUID())
	g.Expect(unstructured.SetNestedField(iref.Object, providerID, "spec", "providerID")).To(Succeed())
	g.Expect(env.Patch(ctx, iref, irefPatch)).To(Succeed())

	irefPatch = client.MergeFrom(iref.DeepCopy())
	g.Expect(unstructured.SetNestedField(iref.Object, true, "status", "initialization", "provisioned")).To(Succeed())
	g.Expect(env.Status().Patch(ctx, iref, irefPatch)).To(Succeed())
	return providerID
}

func fakeMachineNodeRef(m *clusterv1.Machine, pid string, g *WithT) {
	g.Eventually(func() error {
		key := client.ObjectKey{Name: m.Name, Namespace: m.Namespace}
		return env.Get(ctx, key, &clusterv1.Machine{})
	}).Should(Succeed())

	if m.Status.NodeRef.IsDefined() {
		return
	}

	// Create a new fake Node.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: m.Name + "-",
		},
		Spec: corev1.NodeSpec{
			ProviderID: pid,
		},
	}
	g.Expect(env.Create(ctx, node)).To(Succeed())

	g.Eventually(func() error {
		key := client.ObjectKey{Name: node.Name, Namespace: node.Namespace}
		return env.Get(ctx, key, &corev1.Node{})
	}).Should(Succeed())

	// Patch the node and make it look like ready.
	patchNode := client.MergeFrom(node.DeepCopy())
	node.Status.Conditions = append(node.Status.Conditions,
		corev1.NodeCondition{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
		corev1.NodeCondition{Type: corev1.NodePIDPressure, Status: corev1.ConditionFalse},
		corev1.NodeCondition{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
		corev1.NodeCondition{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
	)
	g.Expect(env.Status().Patch(ctx, node, patchNode)).To(Succeed())

	// Patch the Machine.
	patchMachine := client.MergeFrom(m.DeepCopy())
	m.Spec.ProviderID = pid
	g.Expect(env.Patch(ctx, m, patchMachine)).To(Succeed())

	patchMachine = client.MergeFrom(m.DeepCopy())
	m.Status.NodeRef = clusterv1.MachineNodeReference{
		Name: node.Name,
	}
	g.Expect(env.Status().Patch(ctx, m, patchMachine)).To(Succeed())
}
