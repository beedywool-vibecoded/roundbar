const require_index = require("../utils/index.js");
// lib/rules/no-deprecated-dollar-listeners-api.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var no_deprecated_dollar_listeners_api_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow using deprecated `$listeners` (in Vue.js 3.0.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-dollar-listeners-api.html"
		},
		fixable: null,
		schema: [],
		messages: { deprecated: "The `$listeners` is deprecated." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { VExpressionContainer(node) {
			for (const reference of node.references) {
				if (reference.variable != null) continue;
				if (reference.id.name === "$listeners") context.report({
					node: reference.id,
					messageId: "deprecated"
				});
			}
		} }, require_index.default.defineVueVisitor(context, { MemberExpression(node) {
			if (node.property.type !== "Identifier" || node.property.name !== "$listeners" || !require_index.default.isThis(node.object, context)) return;
			context.report({
				node: node.property,
				messageId: "deprecated"
			});
		} }));
	}
};
exports.default = no_deprecated_dollar_listeners_api_default;
