const require_index = require("../utils/index.js");
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/rules/no-watch-after-await.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
function isMaybeUsedStopHandle(node) {
	const parent = node.parent;
	if (parent) switch (parent.type) {
		case "VariableDeclarator": return true;
		case "AssignmentExpression": return true;
		case "CallExpression": return true;
		case "Property": return true;
		case "ArrayExpression": return true;
		case "ChainExpression": return isMaybeUsedStopHandle(parent);
	}
	return false;
}
var no_watch_after_await_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow asynchronously registered `watch`",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-watch-after-await.html"
		},
		fixable: null,
		schema: [],
		messages: { forbidden: "`watch` is forbidden after an `await` expression." }
	},
	create(context) {
		const watchCallNodes = /* @__PURE__ */ new Set();
		const setupScopes = /* @__PURE__ */ new Map();
		let scopeStack = null;
		return require_index.default.compositingVisitors({ Program(program) {
			const tracker = new _eslint_community_eslint_utils.ReferenceTracker(require_index.default.getScope(context, program));
			const watchRefs = require_index.default.iterateReferencesTraceMap(tracker, {
				watch: { [_eslint_community_eslint_utils.ReferenceTracker.CALL]: true },
				watchEffect: { [_eslint_community_eslint_utils.ReferenceTracker.CALL]: true }
			});
			for (const { node } of watchRefs) watchCallNodes.add(node);
		} }, require_index.default.defineVueVisitor(context, {
			onSetupFunctionEnter(node) {
				setupScopes.set(node, {
					afterAwait: false,
					range: node.range
				});
			},
			":function"(node) {
				scopeStack = {
					upper: scopeStack,
					scopeNode: node
				};
			},
			":function:exit"() {
				scopeStack &&= scopeStack.upper;
			},
			AwaitExpression(node) {
				if (!scopeStack) return;
				const setupScope = setupScopes.get(scopeStack.scopeNode);
				if (!setupScope || !require_index.default.isInRange(setupScope.range, node)) return;
				setupScope.afterAwait = true;
			},
			CallExpression(node) {
				if (!scopeStack) return;
				const setupScope = setupScopes.get(scopeStack.scopeNode);
				if (!setupScope || !setupScope.afterAwait || !require_index.default.isInRange(setupScope.range, node)) return;
				if (watchCallNodes.has(node) && !isMaybeUsedStopHandle(node)) context.report({
					node,
					messageId: "forbidden"
				});
			},
			onSetupFunctionExit(node) {
				setupScopes.delete(node);
			}
		}));
	}
};
exports.default = no_watch_after_await_default;
