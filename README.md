
# CaptureOrder

[![Build Status](https://dev.azure.com/theazurechallenge/Kubernetes/_apis/build/status/Code/Azure.azch-captureorder)](https://dev.azure.com/theazurechallenge/Kubernetes/_build/latest?definitionId=10)

A containerised Go swagger API to capture orders, write them to MongoDb and an AMQP message queue.

## Usage

### Swagger

Access the Swagger UI at [http://[host]/swagger]()

### Submitting an order

```
POST /v1/Order HTTP/1.1
Host: [host]:[port]
Content-Type: application/json

{
  "EmailAddress": "test@domain.com",
  "PreferredLanguage": "en"
}
```

## Environment Variables

The following environment variables need to be passed to the container:

### Logging

```
ENV TEAMNAME=[YourTeamName]
ENV CHALLENGEAPPINSIGHTS_KEY=[Challenge Application Insights Key] # Override, if given one by the proctors
```

### For MongoDB

```
ENV MONGOHOST=<mongo service name>.<namespace>
```

```
ENV MONGOUSER=admin
```

```
ENV MONGOPASSWORD=<password for MongoDB>
```

### For CosmosDB

```
ENV MONGOHOST=<cosmosdb account name>.documents.azure.com
```

```
ENV MONGOUSER=<cosmosdb username>
```

```
ENV MONGOPASSWORD=<cosmosdb primary password>
```

## Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
