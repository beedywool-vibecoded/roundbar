const require_index = require("../utils/index.js");
const require_regexp = require("../utils/regexp.js");
const require_html_elements = require("../utils/html-elements.json.js");
const require_deprecated_html_elements = require("../utils/deprecated-html-elements.json.js");
const require_svg_elements = require("../utils/svg-elements.json.js");
const require_vue2_builtin_components = require("../utils/vue2-builtin-components.js");
const require_vue3_builtin_components = require("../utils/vue3-builtin-components.js");
// lib/rules/restricted-component-names.ts
/**
* @author Wayne Zhang
* See LICENSE file in root directory for full license.
*/
const reservedNames = /* @__PURE__ */ new Set([
	...require_html_elements.default,
	...require_deprecated_html_elements.default,
	...require_svg_elements.default,
	...require_vue2_builtin_components.default,
	...require_vue3_builtin_components.default
]);
var restricted_component_names_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce using only specific component names",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/restricted-component-names.html"
		},
		fixable: null,
		schema: [{
			type: "object",
			additionalProperties: false,
			properties: { allow: {
				type: "array",
				items: { type: "string" },
				uniqueItems: true
			} }
		}],
		messages: { invalidName: "Component name \"{{name}}\" is not allowed." }
	},
	create(context) {
		const options = context.options[0] || {};
		const isAllowed = require_regexp.toRegExpGroupMatcher(options.allow);
		function isAllowedTarget(name) {
			return reservedNames.has(name) || isAllowed(name);
		}
		return require_index.default.defineTemplateBodyVisitor(context, { VElement(node) {
			const name = node.rawName;
			if (isAllowedTarget(name)) return;
			context.report({
				node,
				loc: node.loc,
				messageId: "invalidName",
				data: { name }
			});
		} });
	}
};
exports.default = restricted_component_names_default;
