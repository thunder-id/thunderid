// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
)

var scheme = runtime.NewScheme()

const resourceKind = "ThunderIDResource"

func init() {
	utilruntime.Must(appsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
}

// refError is one validation failure against a single ThunderIDResource's field.
type refError struct {
	kind   string
	name   string
	field  string
	ref    string
	detail string
}

// String formats e as "detail" when set, or "not found" when e.ref simply doesn't resolve.
func (e refError) String() string {
	if e.detail != "" {
		return fmt.Sprintf("  [%s/%s] %s %q: %s", e.kind, e.name, e.field, e.ref, e.detail)
	}
	return fmt.Sprintf("  [%s/%s] %s %q not found", e.kind, e.name, e.field, e.ref)
}

func main() {
	allCmd := flag.NewFlagSet("all", flag.ExitOnError)
	allNS := allCmd.String("n", "default", "namespace to validate")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load kubeconfig: %v\n", err)
		os.Exit(1)
	}
	c, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()

	switch os.Args[1] {
	case "all":
		_ = allCmd.Parse(os.Args[2:]) // flag.ExitOnError already exits the process on a parse failure
		runAll(ctx, c, *allNS)
	default:
		runFiles(ctx, c, os.Args[1:])
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  validate all [-n <namespace>]       validate all ThunderIDResource CRs in the cluster")
	fmt.Fprintln(os.Stderr, "  validate <file.yaml> [file2.yaml…]  validate YAML files before applying")
}

// runAll lists every ThunderIDResource CR from the cluster and checks its thunderid.io/instance label
// and spec.resource_type. ThunderIDResource's own body (everything else under spec) is opaque to the
// operator — ThunderID's own bootstrap loader validates it, and errors surface as k8s Events
// read from the pod's logs, not here.
func runAll(ctx context.Context, c client.Client, ns string) {
	list := &appsv1alpha1.ThunderIDResourceList{}
	if err := c.List(ctx, list, client.InNamespace(ns)); err != nil {
		fmt.Fprintf(os.Stderr, "failed to list Resources: %v\n", err)
		os.Exit(1)
	}
	errs := make([]refError, 0, len(list.Items))
	for _, obj := range list.Items {
		errs = append(errs, checkResource(ctx, c, obj.Name, ns, obj.Labels[appsv1alpha1.InstanceRefLabel], obj.Spec)...)
	}

	fmt.Printf("Validating all ThunderIDResource CRs in namespace %q ...\n", ns)
	if len(errs) == 0 {
		fmt.Println("  OK")
	} else {
		for _, e := range errs {
			fmt.Println(e.String())
		}
		fmt.Println()
		fmt.Fprintf(os.Stderr, "%d error(s) found.\n", len(errs))
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println("All references valid.")
}

// runFiles validates YAML files before applying them.
func runFiles(ctx context.Context, c client.Client, files []string) {
	totalErrors := 0
	for _, path := range files {
		fmt.Printf("Validating %s ...\n", path)
		errs := validateFile(ctx, c, path)
		if len(errs) == 0 {
			fmt.Println("  OK")
		} else {
			for _, e := range errs {
				fmt.Println(e.String())
			}
			totalErrors += len(errs)
		}
	}
	fmt.Println()
	if totalErrors > 0 {
		fmt.Fprintf(os.Stderr, "%d error(s) found. Fix these before applying.\n", totalErrors)
		os.Exit(1)
	}
	fmt.Println("All references valid.")
}

// validateFile reads path as one or more "---"-separated YAML documents and runs checkResource
// against every document whose kind is ThunderIDResource, ignoring everything else in the file.
func validateFile(ctx context.Context, c client.Client, path string) []refError {
	data, err := os.ReadFile(path)
	if err != nil {
		return []refError{{kind: "file", name: path, field: "read", detail: err.Error()}}
	}

	var errs []refError
	for doc := range bytes.SplitSeq(data, []byte("\n---")) {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}
		var obj struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name      string            `json:"name"`
				Namespace string            `json:"namespace"`
				Labels    map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec runtime.RawExtension `json:"spec"`
		}
		if err := yaml.Unmarshal(doc, &obj); err != nil {
			errs = append(errs, refError{kind: "file", name: path, field: "yaml", detail: err.Error()})
			continue
		}
		if obj.Kind != resourceKind {
			continue
		}
		ns := obj.Metadata.Namespace
		if ns == "" {
			ns = "default"
		}
		errs = append(errs, checkResource(ctx, c, obj.Metadata.Name, ns,
			obj.Metadata.Labels[appsv1alpha1.InstanceRefLabel], obj.Spec)...)
	}
	return errs
}

// checkResource validates the two k8s-level things the operator itself relies on: the
// thunderid.io/instance label names a real ThunderIDInstance, and spec.resource_type is set.
func checkResource(
	ctx context.Context, c client.Client, name, ns, instanceRef string, spec runtime.RawExtension,
) []refError {
	var errs []refError
	errs = append(errs, checkInstanceRef(ctx, c, name, ns, instanceRef)...)

	var body struct {
		ResourceType string `json:"resource_type"`
	}
	if len(spec.Raw) > 0 {
		_ = yaml.Unmarshal(spec.Raw, &body)
	}
	if body.ResourceType == "" {
		errs = append(errs, refError{
			kind: resourceKind, name: name, field: "spec.resource_type",
			ref: "(empty)", detail: "spec.resource_type is required",
		})
	}
	return errs
}

// checkInstanceRef reports a refError when ref is empty or names a ThunderIDInstance that
// doesn't exist in namespace ns.
func checkInstanceRef(ctx context.Context, c client.Client, name, ns, ref string) []refError {
	if ref == "" {
		return []refError{{
			kind: resourceKind, name: name, field: appsv1alpha1.InstanceRefLabel,
			ref: "(empty)", detail: appsv1alpha1.InstanceRefLabel + " label is required",
		}}
	}
	target := &appsv1alpha1.ThunderIDInstance{}
	err := c.Get(ctx, client.ObjectKey{Namespace: ns, Name: ref}, target)
	if err == nil {
		return nil
	}
	if k8serrors.IsNotFound(err) {
		return []refError{{kind: resourceKind, name: name, field: appsv1alpha1.InstanceRefLabel, ref: ref}}
	}
	return []refError{{
		kind: resourceKind, name: name, field: appsv1alpha1.InstanceRefLabel,
		ref: ref, detail: err.Error(),
	}}
}
