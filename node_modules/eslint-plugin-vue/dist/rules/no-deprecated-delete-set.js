const require_index = require("../utils/index.js");
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/rules/no-deprecated-delete-set.ts
const deletedImportApisMap = {
	set: { [_eslint_community_eslint_utils.ReferenceTracker.CALL]: true },
	del: { [_eslint_community_eslint_utils.ReferenceTracker.CALL]: true }
};
const deprecatedApis = /* @__PURE__ */ new Set(["set", "delete"]);
const deprecatedDollarApis = /* @__PURE__ */ new Set(["$set", "$delete"]);
function isVue(node) {
	return node.type === "Identifier" && node.name === "Vue";
}
var no_deprecated_delete_set_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow using deprecated `$delete` and `$set` (in Vue.js 3.0.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-delete-set.html"
		},
		fixable: null,
		schema: [],
		messages: { deprecated: "The `$delete`, `$set` is deprecated." }
	},
	create(context) {
		function getVueDeprecatedCallExpression(identifier, context) {
			return identifier.parent.type === "MemberExpression" && identifier.parent.parent.type === "CallExpression" && identifier.parent.parent.callee === identifier.parent && (deprecatedDollarApis.has(identifier.name) && require_index.default.isThis(identifier.parent.object, context) || deprecatedApis.has(identifier.name) && isVue(identifier.parent.object)) ? identifier.parent.parent : void 0;
		}
		const nodeVisitor = { Identifier(node) {
			if (!getVueDeprecatedCallExpression(node, context)) return;
			context.report({
				node,
				messageId: "deprecated"
			});
		} };
		return require_index.default.compositingVisitors(require_index.default.defineVueVisitor(context, nodeVisitor), require_index.default.defineScriptSetupVisitor(context, nodeVisitor), { Program(node) {
			const tracker = new _eslint_community_eslint_utils.ReferenceTracker(require_index.default.getScope(context, node));
			const esmTraceMap = { vue: {
				[_eslint_community_eslint_utils.ReferenceTracker.ESM]: true,
				...deletedImportApisMap
			} };
			const cjsTraceMap = { vue: { ...deletedImportApisMap } };
			for (const { node } of [...tracker.iterateEsmReferences(esmTraceMap), ...tracker.iterateCjsReferences(cjsTraceMap)]) {
				const refNode = node;
				context.report({
					node: refNode.callee,
					messageId: "deprecated"
				});
			}
		} });
	}
};
exports.default = no_deprecated_delete_set_default;
