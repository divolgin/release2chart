package helm

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	helmrelease "helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func FindLatestReleaseVersion(namespace string, releaseName string) (int, error) {
	clientSet, err := GetClientset()
	if err != nil {
		return 0, errors.Wrap(err, "get clientset")
	}

	selectorLabels := map[string]string{
		"owner": "helm",
		"name":  releaseName,
	}
	listOpts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(selectorLabels).String(),
	}

	secrets, err := clientSet.CoreV1().Secrets(namespace).List(context.TODO(), listOpts)
	if err != nil {
		return 0, errors.Wrap(err, "list secrets")
	}

	latestRevision := 0
	for _, secret := range secrets.Items {
		revision, err := strconv.Atoi(secret.Labels["version"])
		if err != nil {
			continue
		}

		if revision > latestRevision {
			latestRevision = revision
		}
	}

	return latestRevision, nil
}

func ConvertReleaseVersion(namespace string, releaseName string, revision int) (string, string, error) {
	dstDir := "."

	clientSet, err := GetClientset()
	if err != nil {
		return "", "", errors.Wrap(err, "get clientset")
	}

	selectorLabels := map[string]string{
		"owner":   "helm",
		"name":    releaseName,
		"version": strconv.Itoa(revision),
	}
	listOpts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(selectorLabels).String(),
	}

	secrets, err := clientSet.CoreV1().Secrets(namespace).List(context.TODO(), listOpts)
	if err != nil {
		return "", "", errors.Wrap(err, "list secrets")
	}

	if len(secrets.Items) != 1 {
		return "", "", errors.Errorf("found %d matching releases", len(secrets.Items))
	}

	helmRelease, err := helmReleaseFromReleaseData(secrets.Items[0].Data["release"])
	if err != nil {
		return "", "", errors.Wrap(err, "parse release info from secret")
	}

	releaseDir, err := ioutil.TempDir("", "helm-release-")
	if err != nil {
		return "", "", errors.Wrap(err, "create temp dir")
	}
	defer os.RemoveAll(releaseDir)

	if err := saveReleaseToFiles(helmRelease, releaseDir); err != nil {
		return "", "", errors.Wrap(err, "save release to files")
	}

	client := action.NewPackage()
	client.Destination = dstDir

	chartFile, err := client.Run(releaseDir, nil)
	if err != nil {
		return "", "", errors.Wrap(err, "package client run")
	}

	valuesFile := ""
	if len(helmRelease.Config) != 0 {
		valuesFile = filepath.Join(dstDir, "values.yaml")

		configData, err := yaml.Marshal(helmRelease.Config)
		if err != nil {
			return "", "", errors.Wrap(err, "marshal config data")
		}

		if err = ioutil.WriteFile(valuesFile, configData, 0644); err != nil {
			return "", "", errors.Wrap(err, "write values file")
		}
	}

	return filepath.Base(chartFile), filepath.Base(valuesFile), nil
}

func helmReleaseFromReleaseData(data []byte) (*helmrelease.Release, error) {
	base64Reader := base64.NewDecoder(base64.StdEncoding, bytes.NewReader(data))
	gzreader, err := gzip.NewReader(base64Reader)
	if err != nil {
		return nil, errors.Wrap(err, "create gzip reader")
	}
	defer gzreader.Close()

	releaseData, err := ioutil.ReadAll(gzreader)
	if err != nil {
		return nil, errors.Wrap(err, "read from gzip reader")
	}

	release := &helmrelease.Release{}
	err = json.Unmarshal(releaseData, &release)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal release data")
	}

	return release, nil
}

func saveReleaseToFiles(release *helmrelease.Release, destDir string) error {
	type chartFile struct {
		Name string
		Data []byte
	}

	files := []chartFile{}
	for _, file := range release.Chart.Files {
		files = append(files, chartFile{
			Name: file.Name,
			Data: file.Data,
		})
	}

	for _, template := range release.Chart.Templates {
		files = append(files, chartFile{
			Name: template.Name,
			Data: template.Data,
		})
	}

	chartMetadata, err := yaml.Marshal(release.Chart.Metadata)
	if err != nil {
		return errors.Wrap(err, "marshal chart metadata")
	}
	files = append(files, chartFile{
		Name: "Chart.yaml",
		Data: chartMetadata,
	})

	chartValues, err := yaml.Marshal(release.Chart.Values)
	if err != nil {
		return errors.Wrap(err, "marshal chart values")
	}
	files = append(files, chartFile{
		Name: "values.yaml",
		Data: chartValues,
	})

	chartValuesSchema, err := json.Marshal(release.Chart.Schema)
	if err != nil {
		return errors.Wrap(err, "marshal chart values schema")
	}
	files = append(files, chartFile{
		Name: "values.schema.json",
		Data: chartValuesSchema,
	})

	for _, chartFile := range files {
		fileName := filepath.Join(destDir, chartFile.Name)
		dir := filepath.Dir(fileName)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrapf(err, "create dir %s", dir)
		}

		if err := ioutil.WriteFile(fileName, chartFile.Data, 0644); err != nil {
			return errors.Wrapf(err, "write file %s", fileName)
		}
	}

	return nil
}
