// Bun resolves `with { type: "text" }` imports to the file's contents.
declare module "*.md" {
  const content: string;
  export default content;
}
