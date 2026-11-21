const require_index = require("../utils/index.js");
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/rules/no-lifecycle-after-await.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
const LIFECYCLE_HOOKS = [
	"onBeforeMount",
	"onBeforeUnmount",
	"onBeforeUpdate",
	"onErrorCaptured",
	"onMounted",
	"onRenderTracked",
	"onRenderTriggered",
	"onUnmounted",
	"onUpdated",
	"onActivated",
	"onDeactivated"
];
var no_lifecycle_after_await_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow asynchronously registered lifecycle hooks",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-lifecycle-after-await.html"
		},
		fixable: null,
		schema: [],
		messages: { forbidden: "Lifecycle hooks are forbidden after an `await` expression." }
	},
	create(context) {
		const lifecycleHookCallNodes = /* @__PURE__ */ new Set();
		const setupScopes = /* @__PURE__ */ new Map();
		let scopeStack = null;
		return require_index.default.compositingVisitors({ Program(program) {
			const tracker = new _eslint_community_eslint_utils.ReferenceTracker(require_index.default.getScope(context, program));
			const traceMap = {};
			for (const lifecycleHook of LIFECYCLE_HOOKS) traceMap[lifecycleHook] = { [_eslint_community_eslint_utils.ReferenceTracker.CALL]: true };
			for (const { node } of require_index.default.iterateReferencesTraceMap(tracker, traceMap)) lifecycleHookCallNodes.add(node);
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
				if (!setupScope?.afterAwait || !require_index.default.isInRange(setupScope.range, node) || !lifecycleHookCallNodes.has(node) || node.arguments.length >= 2) return;
				context.report({
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
exports.default = no_lifecycle_after_await_default;
