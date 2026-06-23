// Structured image edits compiled to server-side yq-i expressions. The client
// never re-serializes YAML: the mutation runs in ConfigHub's function executor
// (comment- and format-preserving) and the preview comes from a dry run.
// Validated offline against the example manifests with `cub function local`.

export interface CompiledEdit {
  /** yq expression for the yq-i function. */
  expr: string;
  /** Human summary, used as the default change description. */
  summary: string;
}

/** Repin a container's image. `image:` is matched by container name so a
 * multi-container pod only repoints the one container. */
export function compileSetImage(containerName: string, image: string): CompiledEdit {
  const sel = `select(.kind == "Deployment").spec.template.spec.containers[] | select(.name == "${containerName}")`;
  return {
    expr: `(${sel}).image = "${image}"`,
    summary: `Set image of container "${containerName}" to ${image}`,
  };
}
