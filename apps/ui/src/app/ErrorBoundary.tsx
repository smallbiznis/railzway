import { isRouteErrorResponse, useRouteError } from "react-router-dom";

export default function ErrorBoundary() {
  const error = useRouteError();

  if (isRouteErrorResponse(error)) {
    return (
      <div className="h-screen flex items-center justify-center">
        <div className="max-w-md text-center">
          <h1 className="text-2xl font-bold">
            {error.status} – {error.statusText}
          </h1>
          <p className="mt-2 text-text-muted">
            {error.data || "Something went wrong"}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-screen flex items-center justify-center">
      <div className="max-w-md text-center">
        <h1 className="text-2xl font-bold">Unexpected error</h1>
        <p className="mt-2 text-text-muted">
          {(error as Error)?.message ?? "Unknown error"}
        </p>
      </div>
    </div>
  );
}
