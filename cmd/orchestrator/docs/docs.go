// Code generated by swaggo/swag. DO NOT EDIT
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/aws2k8s": {
            "post": {
                "description": "Triggers a run of the AWS2K8s Job and returns success",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "aws2k8s"
                ],
                "summary": "Trigger a run of the AWS2K8s Job",
                "responses": {
                    "201": {
                        "description": "Created"
                    },
                    "404": {
                        "description": "Not Found"
                    },
                    "409": {
                        "description": "Conflict"
                    },
                    "500": {
                        "description": "Internal Server Error"
                    }
                }
            }
        },
        "/awsmapping": {
            "post": {
                "description": "Triggers a run of the AwsMapping Job and returns success",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "awsmapping"
                ],
                "summary": "Trigger a run of the AwsMapping Job",
                "responses": {
                    "201": {
                        "description": "Created"
                    },
                    "404": {
                        "description": "Not Found"
                    },
                    "409": {
                        "description": "Conflict"
                    },
                    "500": {
                        "description": "Internal Server Error"
                    }
                }
            }
        },
        "/azure2aws": {
            "post": {
                "description": "Triggers a run of the Azure2AWS Job and returns success",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "azure2aws"
                ],
                "summary": "Trigger a run of the Azure2AWS Job",
                "responses": {
                    "201": {
                        "description": "Created"
                    },
                    "404": {
                        "description": "Not Found"
                    },
                    "409": {
                        "description": "Conflict"
                    },
                    "500": {
                        "description": "Internal Server Error"
                    }
                }
            }
        },
        "/capsvc2azure": {
            "post": {
                "description": "Triggers a run of the CapSvc2Azure Job and returns success",
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "capsvc2azure"
                ],
                "summary": "Trigger a run of the CapSvc2Azure Job",
                "responses": {
                    "201": {
                        "description": "Created"
                    },
                    "404": {
                        "description": "Not Found"
                    },
                    "409": {
                        "description": "Conflict"
                    },
                    "500": {
                        "description": "Internal Server Error"
                    }
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "1",
	Host:             "",
	BasePath:         "/api/v1",
	Schemes:          []string{},
	Title:            "AAD AWS Sync",
	Description:      "",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
