# Thoth
![thoth](https://cloud.githubusercontent.com/assets/223760/6539313/ecbb51cc-c425-11e4-92e9-8515b6f1c9ef.png)

An App responsible for measuring and controlling time

## Setup


### Push Benchmarked App

The benchmarked app consists of an empty index.html plus an ngninx config and is pushed using the following:
```
cf push benchmarked-app -p benchmarked-app -n <benchmarked-app-hostname> -i 2 -m 64M -b https://github.com/cloudfoundry-community/staticfile-buildpack.git
```

### Push Thoth

```
cf push thoth --no-route --no-start
cf set-env thoth CF_APP_NAME <benchmarked-app-name>
cf set-env thoth CF_DEPLOYMENT_NAME <your-deployment-name>
cf set-env thoth CF_ORG <your-org-name>
cf set-env thoth CF_PASSWORD <your-password>
cf set-env thoth CF_SKIP_SSL_VALIDATION <true/false>
cf set-env thoth CF_SPACE <your-space>
cf set-env thoth CF_SYSTEM_DOMAIN <cf-system-domain>
cf set-env thoth CF_USERNAME <your-username>
cf set-env thoth DATADOG_API_KEY <your-datadog-api-key>

# optionally set the number of concurrent benchmarks
cf set-env thoth THOTH_THREADS 5

cf start thoth
```

## Metrics (from the bottom up)

![metrics](https://cloud.githubusercontent.com/assets/223760/6404049/d3c167c8-bdc8-11e4-8a15-11cfed863565.png)

* Time in App (`app_benchmarking.time_in_app`)
* Time in Gorouter (`app_benchmarking.time_in_gorouter`)
* Rest of Time (`app_benchmarking.rest_of_time`)
