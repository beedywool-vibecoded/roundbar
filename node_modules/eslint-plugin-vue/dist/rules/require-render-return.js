const require_index = require("../utils/index.js");
// lib/rules/require-render-return.ts
/**
* @fileoverview Enforces render function to always return value.
* @author Armano
*/
var require_render_return_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce render function to always return value",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/require-render-return.html"
		},
		fixable: null,
		schema: [],
		messages: { expectedReturn: "Expected to return a value in render function." }
	},
	create(context) {
		const renderFunctions = /* @__PURE__ */ new Map();
		return require_index.default.compositingVisitors(require_index.default.defineVueVisitor(context, { onRenderFunctionEnter(node) {
			renderFunctions.set(node, node.parent.key);
		} }), require_index.default.executeOnFunctionsWithoutReturn(context, true, (node) => {
			const key = renderFunctions.get(node);
			if (key) context.report({
				node: key,
				messageId: "expectedReturn"
			});
		}));
	}
};
exports.default = require_render_return_default;
