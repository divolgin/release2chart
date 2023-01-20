# release2chart

Convert a deployed release to an installable Helm chart with original values file.

Example:

```
./bin/release2chart postgresql -n divolgin

To install the converted chart, run the following command:
helm install postgresql postgresql-8.1.40.tgz --values values.yaml --namespace divolgin
```