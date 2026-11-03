import { useState } from "react";
import { ApiError } from "@/lib/api";

/** Maps a 422 `details` object from the API onto form fields. */
export function useFieldErrors() {
  const [errors, setErrors] = useState<Record<string, string>>({});
  return {
    errors,
    clear: () => setErrors({}),
    /** Returns true when the error was fully field-level and was absorbed into the form. */
    capture: (err: unknown): boolean => {
      if (err instanceof ApiError && err.status === 422 && err.details) {
        setErrors(err.details);
        return true;
      }
      return false;
    },
  };
}
