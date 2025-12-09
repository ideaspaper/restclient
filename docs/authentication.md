# Authentication

## Basic Authentication

```http
GET https://api.example.com/protected
Authorization: Basic username:password
```

Or pre-encoded:

```http
GET https://api.example.com/protected
Authorization: Basic dXNlcm5hbWU6cGFzc3dvcmQ=
```

## Digest Authentication

```http
GET https://api.example.com/protected
Authorization: Digest username password
```

## AWS Signature v4

```http
GET https://s3.us-east-1.amazonaws.com/my-bucket
Authorization: AWS accessKeyId secretAccessKey
```

With optional parameters:

```http
GET https://api.example.com/resource
Authorization: AWS accessKeyId secretAccessKey token:sessionToken region:us-west-2 service:execute-api
```
