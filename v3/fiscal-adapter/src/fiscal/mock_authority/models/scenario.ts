export type MockScenario =
  | "authorized"
  | "rejected"
  | "timeout_before_processing"
  | "response_lost_after_processing";
