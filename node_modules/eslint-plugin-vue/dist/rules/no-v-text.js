const require_index = require("../utils/index.js");
// lib/rules/no-v-text.ts
/**
* @author tyankatsu <https://github.com/tyankatsu0105>
* See LICENSE file in root directory for full license.
*/
var no_v_text_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow use of v-text",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-v-text.html"
		},
		fixable: null,
		schema: [],
		messages: { unexpected: "Don't use 'v-text'." }
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='text']"(node) {
			context.report({
				node,
				loc: node.loc,
				messageId: "unexpected"
			});
		} });
	}
};
exports.default = no_v_text_default;
