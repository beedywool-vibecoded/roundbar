const require_index = require("../utils/index.js");
const require_slot_attribute = require("./syntaxes/slot-attribute.js");
// lib/rules/no-deprecated-slot-attribute.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var no_deprecated_slot_attribute_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow deprecated `slot` attribute (in Vue.js 2.6.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-slot-attribute.html"
		},
		fixable: "code",
		schema: [{
			type: "object",
			properties: {
				ignore: {
					type: "array",
					items: { type: "string" },
					uniqueItems: true
				},
				ignoreParents: {
					type: "array",
					items: { type: "string" },
					uniqueItems: true
				}
			},
			additionalProperties: false
		}],
		messages: { forbiddenSlotAttribute: "`slot` attributes are deprecated." }
	},
	create(context) {
		const templateBodyVisitor = require_slot_attribute.default.createTemplateBodyVisitor(context);
		return require_index.default.defineTemplateBodyVisitor(context, templateBodyVisitor);
	}
};
exports.default = no_deprecated_slot_attribute_default;
