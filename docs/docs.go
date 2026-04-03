package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "swagger": "2.0",
    "info": {
        "title": "Op-Bot API",
        "description": "Portfolio theme deployer with GitHub OAuth and resume validation",
        "version": "1.0"
    },
    "host": "localhost:8080",
    "basePath": "/",
    "paths": {
        "/auth/github/start": {
            "get": {
                "tags": ["auth"],
                "summary": "Start GitHub OAuth flow",
                "description": "Redirects to GitHub for OAuth authorization",
                "parameters": [
                    {
                        "name": "returnTo",
                        "in": "query",
                        "type": "string",
                        "description": "URL to return to after auth"
                    }
                ],
                "responses": {
                    "302": {
                        "description": "Redirect to GitHub"
                    },
                    "500": {
                        "description": "Server error",
                        "schema": {
                            "$ref": "#/definitions/errorResponse"
                        }
                    }
                }
            }
        },
        "/auth/github/callback": {
            "get": {
                "tags": ["auth"],
                "summary": "GitHub OAuth callback",
                "description": "Handles OAuth callback from GitHub and sets auth cookie",
                "parameters": [
                    {
                        "name": "code",
                        "in": "query",
                        "required": true,
                        "type": "string",
                        "description": "OAuth code"
                    },
                    {
                        "name": "state",
                        "in": "query",
                        "required": true,
                        "type": "string",
                        "description": "OAuth state"
                    }
                ],
                "responses": {
                    "302": {
                        "description": "Redirect to return URL"
                    },
                    "400": {
                        "description": "Invalid OAuth state",
                        "schema": {
                            "$ref": "#/definitions/errorResponse"
                        }
                    },
                    "500": {
                        "description": "OAuth callback failed",
                        "schema": {
                            "$ref": "#/definitions/errorResponse"
                        }
                    }
                }
            }
        },
        "/api/github/me": {
            "get": {
                "tags": ["github"],
                "summary": "Get current GitHub user",
                "description": "Returns the authenticated GitHub user and app installation info",
                "produces": ["application/json"],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/meResponse"
                        }
                    },
                    "401": {
                        "description": "Unauthorized",
                        "schema": {
                            "$ref": "#/definitions/errorResponse"
                        }
                    }
                }
            }
        },
        "/api/github/logout": {
            "post": {
                "tags": ["github"],
                "summary": "Logout from GitHub",
                "description": "Clears the GitHub auth cookie",
                "responses": {
                    "204": {
                        "description": "No Content"
                    }
                }
            }
        },
        "/api/resume/validate": {
            "post": {
                "tags": ["resume"],
                "summary": "Validate resume JSON",
                "description": "Validates resume data against the RxResume schema",
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "parameters": [
                    {
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/validateRequest"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/validationResult"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/errorResponse"
                        }
                    }
                }
            }
        },
        "/api/github/deploy": {
            "post": {
                "tags": ["github"],
                "summary": "Deploy portfolio theme",
                "description": "Creates a GitHub repository and deploys the selected theme",
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "parameters": [
                    {
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/deployParams"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/deployResult"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "$ref": "#/definitions/errorResponse"
                        }
                    },
                    "401": {
                        "description": "Unauthorized",
                        "schema": {
                            "$ref": "#/definitions/errorResponse"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "errorResponse": {
            "type": "object",
            "properties": {
                "error": {
                    "type": "object",
                    "properties": {
                        "code": {
                            "type": "string",
                            "example": "BAD_REQUEST"
                        },
                        "message": {
                            "type": "string",
                            "example": "Invalid request body"
                        },
                        "details": {
                            "type": "array",
                            "items": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "validationResult": {
            "type": "object",
            "properties": {
                "valid": {
                    "type": "boolean",
                    "example": true
                },
                "errors": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                }
            }
        },
        "validateRequest": {
            "type": "object",
            "properties": {
                "resumeData": {
                    "type": "object",
                    "description": "Resume JSON data to validate"
                }
            }
        },
        "deployParams": {
            "type": "object",
            "properties": {
                "theme": {
                    "type": "string",
                    "example": "modern",
                    "enum": ["modern", "graphic", "newspaper", "vscode"]
                },
                "repositoryName": {
                    "type": "string",
                    "example": "my-portfolio"
                },
                "privateRepo": {
                    "type": "boolean",
                    "example": false
                },
                "resumeData": {
                    "type": "object",
                    "description": "Optional resume JSON data"
                }
            }
        },
        "deployResult": {
            "type": "object",
            "properties": {
                "repositoryUrl": {
                    "type": "string",
                    "example": "https://github.com/user/my-portfolio"
                },
                "pagesUrl": {
                    "type": "string",
                    "example": "https://user.github.io/my-portfolio/"
                },
                "repoFullName": {
                    "type": "string",
                    "example": "user/my-portfolio"
                },
                "reusedExistingRepo": {
                    "type": "boolean",
                    "example": false
                },
                "installationId": {
                    "type": "integer",
                    "example": 12345
                }
            }
        },
        "gitHubUser": {
            "type": "object",
            "properties": {
                "login": {
                    "type": "string",
                    "example": "octocat"
                },
                "name": {
                    "type": "string",
                    "example": "The Octocat"
                },
                "avatarUrl": {
                    "type": "string",
                    "example": "https://github.com/images/octocat.png"
                }
            }
        },
        "gitHubAppInfo": {
            "type": "object",
            "properties": {
                "installed": {
                    "type": "boolean",
                    "example": true
                },
                "installationId": {
                    "type": "integer",
                    "example": 12345
                },
                "installUrl": {
                    "type": "string",
                    "example": "https://github.com/apps/myapp/installations/new"
                }
            }
        },
        "meResponse": {
            "type": "object",
            "properties": {
                "user": {
                    "$ref": "#/definitions/gitHubUser"
                },
                "githubApp": {
                    "$ref": "#/definitions/gitHubAppInfo"
                }
            }
        }
    }
}`

var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "localhost:8080",
	BasePath:         "/",
	Schemes:          []string{},
	Title:            "Op-Bot API",
	Description:      "Portfolio theme deployer with GitHub OAuth and resume validation",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
