export interface Entry {
  EventType: number;
  ActionType?: string;
  SessionID?: string;
  Details?: string;
  OTR?: boolean;
  Source?: string;
}

export interface LogEntry {
  ID: number;
  Timestamp: string;
  EventType: number;
  ActionType: string;
  SessionID: string;
  Details: string;
  Hash: string;
  PrevHash: string;
}

export interface Query {
  SessionID?: string;
  EventType?: number;
  ActionType?: string;
  Limit?: number;
}

export interface VerifyResult {
  valid: boolean;
  error?: string;
}
