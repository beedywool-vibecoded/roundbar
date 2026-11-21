const require_index = require("../utils/index.js");
const require_slot_scope_attribute = require("./syntaxes/slot-scope-attribute.js");
// lib/rules/no-deprecated-slot-scope-attribute.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var no_deprecated_slot_scope_attribute_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow deprecated `slot-scope` attribute (in Vue.js 2.6.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-slot-scope-attribute.html"
		},
		fixable: "code",
		schema: [],
		messages: { forbiddenSlotScopeAttribute: "`slot-scope` are deprecated." }
	},
	create(context) {
		const templateBodyVisitor = require_slot_scope_attribute.default.createTemplateBodyVisitor(context, { fixToUpgrade: true });
		return require_index.default.defineTemplateBodyVisitor(context, templateBodyVisitor);
	}
};
exports.default = no_deprecated_slot_scope_attribute_default;
