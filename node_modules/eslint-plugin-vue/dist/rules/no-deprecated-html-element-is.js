const require_index = require("../utils/index.js");
// lib/rules/no-deprecated-html-element-is.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var no_deprecated_html_element_is_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow using deprecated the `is` attribute on HTML elements (in Vue.js 3.0.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-html-element-is.html"
		},
		fixable: null,
		schema: [],
		messages: { unexpected: "The `is` attribute on HTML element are deprecated." }
	},
	create(context) {
		function isValidElement(node) {
			return !require_index.default.isHtmlWellKnownElementName(node.rawName) && !require_index.default.isSvgWellKnownElementName(node.rawName) && !require_index.default.isMathWellKnownElementName(node.rawName);
		}
		return require_index.default.defineTemplateBodyVisitor(context, {
			"VAttribute[directive=true][key.name.name='bind'][key.argument.name='is']"(node) {
				if (isValidElement(node.parent.parent)) return;
				context.report({
					node,
					loc: node.loc,
					messageId: "unexpected"
				});
			},
			"VAttribute[directive=false][key.name='is']"(node) {
				if (isValidElement(node.parent.parent) || node.value?.value.startsWith("vue:")) return;
				context.report({
					node,
					loc: node.loc,
					messageId: "unexpected"
				});
			}
		});
	}
};
exports.default = no_deprecated_html_element_is_default;
