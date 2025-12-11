# Documentation development

OSCAR uses [MKDocs](https://www.mkdocs.org) for the documentation. In particular, [Material for MKDocs](https://squidfunk.github.io/mkdocs-material/).

Install the following dependencies:

```sh
pip install mkdocs mkdocs-material mkdocs-render-swagger-plugin mike
```

The from the main folder `oscar` run:

```sh
mkdocs serve
```

The documentation will be available in [http://127.0.0.1:8000](http://127.0.0.1:8000)

## API documentation

To generate the OpenAPI spec from swagger comments:

```sh
swag init -g main.go -o pkg/apidocs
```

To serve the generated `swagger.json` locally with Swagger UI:

```sh
docker run --rm -p 8081:8080 \
  -e SWAGGER_JSON=/docs/swagger.json \
  -v "$(pwd)/pkg/apidocs/swagger.json:/docs/swagger.json" \
  swaggerapi/swagger-ui
```

Then open http://localhost:8081 to browse the API.
