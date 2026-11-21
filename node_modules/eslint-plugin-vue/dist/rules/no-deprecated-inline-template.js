const require_index = require("../utils/index.js");
// lib/rules/no-deprecated-inline-template.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var no_deprecated_inline_template_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow using deprecated `inline-template` attribute (in Vue.js 3.0.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-inline-template.html"
		},
		fixable: null,
		schema: [],
		messages: { unexpected: "`inline-template` are deprecated." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=false] > VIdentifier[rawName='inline-template']"(node) {
			context.report({
				node,
				loc: node.loc,
				messageId: "unexpected"
			});
		} });
	}
};
exports.default = no_deprecated_inline_template_default;
