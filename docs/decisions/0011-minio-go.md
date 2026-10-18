# 0011. minio-go for object storage

Status: accepted
Date: 2026-10-18

## Context

Photos, logos and PDFs live in S3-compatible storage (OVHcloud Object Storage).

## Decision

Use `github.com/minio/minio-go/v7` as the S3 client: maintained, widely used, Apache-2.0 licensed, lighter than the AWS SDK.

## Consequences

- A filesystem backend serves dev and tests (`AWT_BLOB_BACKEND=fs`).
- Added with a one-line justification in the commit that first imports it.
