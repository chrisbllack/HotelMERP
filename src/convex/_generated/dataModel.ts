// Stub file for Convex generated dataModel types
// This is a placeholder until convex dev generates the real types

export type Id<TableName extends string> = string & { __table: TableName };
export type Doc<TableName extends string> = { _id: Id<TableName> } & Record<string, any>;