# Thoth
An App responsible for measuring and controlling time

## Pushing

```
cf push thoth --no-route --no-start
cf set-env thoth CF_APP_NAME <your value here>
cf set-env thoth CF_DEPLOYMENT_NAME <your value here>
cf set-env thoth CF_ORG <your value here>
cf set-env thoth CF_PASSWORD <your value here>
cf set-env thoth CF_SKIP_SSL_VALIDATION <your value here>
cf set-env thoth CF_SPACE <your value here>
cf set-env thoth CF_SYSTEM_DOMAIN <your value here>
cf set-env thoth CF_USERNAME <your value here>
cf set-env thoth DATADOG_API_KEY <your value here>
cf start thoth
```
