package ui

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"strings"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// explained is a failure in plain words: a short title, one sentence on what
// it means, and the raw error for a tooltip.
type explained struct {
	Title  string
	Detail string
	Raw    string
}

// explainError maps R2, network and local failures to plain words, in the
// spirit of echohub's explain_error.
func explainError(err error) explained {
	if err == nil {
		return explained{}
	}
	out := explained{Raw: err.Error()}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "InvalidAccessKeyId", "SignatureDoesNotMatch", "Unauthorized", "InvalidToken":
			out.Title, out.Detail = "Credentials rejected", "R2 did not accept this account's access key or secret. Edit the account and enter them again."
		case "AccessDenied", "Forbidden":
			out.Title, out.Detail = "Access denied", "This access key is not allowed to do that. Check the token's permissions and bucket scope in Cloudflare."
		case "NoSuchBucket":
			out.Title, out.Detail = "Bucket not found", "The bucket no longer exists or the account cannot see it. Refresh the bucket list."
		case "NoSuchKey", "NotFound":
			out.Title, out.Detail = "Object not found", "It was moved or deleted. Refresh the folder to see its current contents."
		case "SlowDown", "TooManyRequests":
			out.Title, out.Detail = "Rate limited", "R2 asked r2go to slow down. Wait a moment and try again."
		case "InternalError", "ServiceUnavailable":
			out.Title, out.Detail = "R2 server error", "Cloudflare had a problem handling the request. Try again shortly."
		}
		if out.Title != "" {
			return out
		}
	}

	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) {
		switch status := responseErr.HTTPStatusCode(); {
		case status == http.StatusUnauthorized:
			out.Title, out.Detail = "Credentials rejected", "R2 did not accept this account's access key or secret."
		case status == http.StatusForbidden:
			out.Title, out.Detail = "Access denied", "This access key is not allowed to do that."
		case status == http.StatusNotFound:
			out.Title, out.Detail = "Not found", "It was moved or deleted, or the account cannot see it."
		case status == http.StatusTooManyRequests:
			out.Title, out.Detail = "Rate limited", "R2 asked r2go to slow down. Wait a moment and try again."
		case status >= 500:
			out.Title, out.Detail = "R2 server error", "Cloudflare had a problem handling the request. Try again shortly."
		}
		if out.Title != "" {
			return out
		}
	}

	var dnsErr *net.DNSError
	var netErr net.Error
	switch {
	case errors.Is(err, context.Canceled):
		out.Title, out.Detail = "Cancelled", "The operation was stopped before it finished."
	case errors.Is(err, context.DeadlineExceeded):
		out.Title, out.Detail = "Not responding", "R2 took too long to answer. Check your connection and try again."
	case errors.As(err, &dnsErr):
		out.Title, out.Detail = "Can't find R2", "The R2 endpoint name could not be resolved. Check your connection and the account ID."
	case errors.As(err, &netErr) && netErr.Timeout():
		out.Title, out.Detail = "Not responding", "R2 took too long to answer. Check your connection and try again."
	case errors.As(err, &netErr), strings.Contains(err.Error(), "connection refused"), strings.Contains(err.Error(), "no such host"):
		out.Title, out.Detail = "Can't reach R2", "r2go could not connect to Cloudflare. Check your connection."
	case errors.Is(err, fs.ErrPermission):
		out.Title, out.Detail = "Permission denied", "This computer did not allow r2go to read or write that file or folder."
	case errors.Is(err, fs.ErrNotExist):
		out.Title, out.Detail = "File not found", "The local file or folder is no longer there."
	case strings.Contains(err.Error(), "no R2 secret stored"):
		out.Title, out.Detail = "Secret missing", "This account has no saved secret access key. Edit the account and enter it again."
	default:
		out.Title, out.Detail = "Something went wrong", firstLine(err.Error())
	}
	return out
}

func firstLine(value string) string {
	if i := strings.IndexByte(value, '\n'); i >= 0 {
		value = value[:i]
	}
	if len(value) > 160 {
		value = value[:157] + "…"
	}
	return value
}
