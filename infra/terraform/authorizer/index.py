# REQUEST authorizer for the experiment API (api_lambda.tf). The API accepts a call only from CloudFront, which sends
# the shared secret in x-origin-verify (frontend.tf). Constant-time comparison; no identity, no caching.
import hmac
import os


def handler(event, context):
    sent = (event.get("headers") or {}).get("x-origin-verify", "")
    return {"isAuthorized": hmac.compare_digest(sent, os.environ["ORIGIN_VERIFY"])}
