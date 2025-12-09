# HTTP File Format

## Basic Request

```http
GET https://api.example.com/users
```

## With Headers

```http
GET https://api.example.com/users
Authorization: Bearer my-token
Accept: application/json
Content-Type: application/json
```

## With Body

```http
POST https://api.example.com/users
Content-Type: application/json

{
    "name": "John Doe",
    "email": "john@example.com"
}
```

## Multiple Requests

Separate requests with `###`:

```http
### Get users
GET https://api.example.com/users

### Create user
POST https://api.example.com/users
Content-Type: application/json

{"name": "John"}

### Delete user
DELETE https://api.example.com/users/123
```

## Request Metadata

Use comments with `@` prefix for metadata:

```http
# @name myRequest
# @note This is a test request
# @no-redirect
# @no-cookie-jar
GET https://api.example.com/users
```

| Metadata         | Description                    |
| ---------------- | ------------------------------ |
| `@name`          | Name the request for reference |
| `@note`          | Add a description              |
| `@no-redirect`   | Don't follow redirects         |
| `@no-cookie-jar` | Don't use cookie jar           |
| `@prompt`        | Define prompt variables        |

## Query Parameters

Multi-line query parameters:

```http
GET https://api.example.com/users
    ?page=1
    &limit=10
    &sort=name
    &order=asc
```

## Form URL Encoded

```http
POST https://api.example.com/login
Content-Type: application/x-www-form-urlencoded

username=john
&password=secret123
&remember=true
```

## Multipart Form Data

```http
POST https://api.example.com/upload
Content-Type: multipart/form-data; boundary=----FormBoundary

------FormBoundary
Content-Disposition: form-data; name="title"

My Document
------FormBoundary
Content-Disposition: form-data; name="file"; filename="document.pdf"
Content-Type: application/pdf

< ./document.pdf
------FormBoundary--
```

## File References

Include file contents in request body:

```http
POST https://api.example.com/upload
Content-Type: application/json

< ./data.json
```

## GraphQL

```http
POST https://api.example.com/graphql
Content-Type: application/json
X-Request-Type: GraphQL

query GetUser($id: ID!) {
    user(id: $id) {
        name
        email
    }
}

{"id": "123"}
```

GraphQL is auto-detected for URLs ending in `/graphql`:

```http
POST https://api.example.com/graphql
Content-Type: application/json

mutation CreateUser($input: CreateUserInput!) {
    createUser(input: $input) {
        id
        name
    }
}

{"input": {"name": "John", "email": "john@example.com"}}
```
