// lib/utils/scope.ts
/**
* Gets the scope for the current node
*/
function getScope(context, currentNode) {
	const isInner = currentNode.type !== "Program";
	const scopeManager = context.sourceCode.scopeManager;
	let node = currentNode;
	for (; node; node = node.parent) {
		const scope = scopeManager.acquire(node, isInner);
		if (scope) return scope.type === "function-expression-name" ? scope.childScopes[0] : scope;
	}
	return scopeManager.scopes[0];
}
exports.getScope = getScope;
