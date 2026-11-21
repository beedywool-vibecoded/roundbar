const require_index = require("../utils/index.js");
// lib/rules/no-deprecated-filter.ts
/**
* @author Przemyslaw Falowski (@przemkow)
* @fileoverview disallow using deprecated filters syntax
*/
var no_deprecated_filter_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow using deprecated filters syntax (in Vue.js 3.0.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-filter.html"
		},
		fixable: null,
		schema: [],
		messages: { noDeprecatedFilter: "Filters are deprecated." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { VFilterSequenceExpression(node) {
			context.report({
				node,
				loc: node.loc,
				messageId: "noDeprecatedFilter"
			});
		} });
	}
};
exports.default = no_deprecated_filter_default;
