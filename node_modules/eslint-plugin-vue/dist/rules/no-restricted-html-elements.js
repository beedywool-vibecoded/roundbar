const require_index = require("../utils/index.js");
// lib/rules/no-restricted-html-elements.ts
/**
* @author Doug Wade <douglas.b.wade@gmail.com>
*/
var no_restricted_html_elements_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow specific elements",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-restricted-html-elements.html"
		},
		fixable: null,
		schema: {
			type: "array",
			items: { oneOf: [{ type: "string" }, {
				type: "object",
				properties: {
					element: { oneOf: [{ type: "string" }, {
						type: "array",
						items: { type: "string" }
					}] },
					message: {
						type: "string",
						minLength: 1
					}
				},
				required: ["element"],
				additionalProperties: false
			}] },
			uniqueItems: true,
			minItems: 0
		},
		messages: {
			forbiddenElement: "Unexpected use of forbidden element {{name}}.",
			customMessage: "{{message}}"
		}
	},
	create(context) {
		return require_index.default.defineTemplateBodyVisitor(context, { VElement(node) {
			if (!require_index.default.isHtmlElementNode(node) && !require_index.default.isSvgElementNode(node) && !require_index.default.isMathElementNode(node)) return;
			for (const option of context.options) {
				const restrictedItem = option.element || option;
				if ((Array.isArray(restrictedItem) ? restrictedItem : [restrictedItem]).includes(node.rawName)) {
					context.report({
						messageId: option.message ? "customMessage" : "forbiddenElement",
						data: {
							name: node.rawName,
							message: option.message
						},
						node: node.startTag
					});
					return;
				}
			}
		} });
	}
};
exports.default = no_restricted_html_elements_default;
