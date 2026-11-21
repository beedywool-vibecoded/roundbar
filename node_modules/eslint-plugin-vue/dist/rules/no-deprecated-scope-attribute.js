const require_index = require("../utils/index.js");
const require_scope_attribute = require("./syntaxes/scope-attribute.js");
// lib/rules/no-deprecated-scope-attribute.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var no_deprecated_scope_attribute_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow deprecated `scope` attribute (in Vue.js 2.5.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-scope-attribute.html"
		},
		fixable: "code",
		schema: [],
		messages: { forbiddenScopeAttribute: "`scope` attributes are deprecated." }
	},
	create(context) {
		const templateBodyVisitor = require_scope_attribute.default.createTemplateBodyVisitor(context);
		return require_index.default.defineTemplateBodyVisitor(context, templateBodyVisitor);
	}
};
exports.default = no_deprecated_scope_attribute_default;
