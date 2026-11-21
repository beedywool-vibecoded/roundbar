const require_index = require("../utils/index.js");
const require_casing = require("../utils/casing.js");
const require_html_elements = require("../utils/html-elements.json.js");
const require_deprecated_html_elements = require("../utils/deprecated-html-elements.json.js");
const require_svg_elements = require("../utils/svg-elements.json.js");
// lib/rules/no-reserved-component-names.ts
/**
* @fileoverview disallow the use of reserved names in component definitions
* @author Jake Hassel <https://github.com/shadskii>
*/
const kebabCaseElements = [
	"annotation-xml",
	"color-profile",
	"font-face",
	"font-face-src",
	"font-face-uri",
	"font-face-format",
	"font-face-name",
	"missing-glyph"
];
function isLowercase(word) {
	return /^[a-z]*$/.test(word);
}
const RESERVED_NAMES_IN_HTML = new Set(require_html_elements.default);
const RESERVED_NAMES_IN_OTHERS = /* @__PURE__ */ new Set([
	...require_deprecated_html_elements.default,
	...kebabCaseElements,
	...require_svg_elements.default
]);
const CAPITALIZED_RESERVED_NAMES_IN_HTML = new Set(require_html_elements.default.map(require_casing.capitalize));
const CAPITALIZED_RESERVED_NAMES_IN_OTHERS = /* @__PURE__ */ new Set([
	...require_deprecated_html_elements.default.map(require_casing.capitalize),
	...kebabCaseElements.map(require_casing.pascalCase),
	...require_svg_elements.default.filter(isLowercase).map(require_casing.capitalize)
]);
var no_reserved_component_names_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow the use of reserved names in component definitions",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-reserved-component-names.html"
		},
		fixable: null,
		schema: [{
			type: "object",
			properties: {
				disallowVueBuiltInComponents: { type: "boolean" },
				disallowVue3BuiltInComponents: { type: "boolean" },
				htmlElementCaseSensitive: { type: "boolean" }
			},
			additionalProperties: false
		}],
		messages: {
			reserved: "Name \"{{name}}\" is reserved.",
			reservedInHtml: "Name \"{{name}}\" is reserved in HTML.",
			reservedInVue: "Name \"{{name}}\" is reserved in Vue.js.",
			reservedInVue3: "Name \"{{name}}\" is reserved in Vue.js 3.x."
		}
	},
	create(context) {
		const options = context.options[0] || {};
		const shouldDisallowVueBuiltInComponents = options.disallowVueBuiltInComponents === true;
		const shouldDisallowVue3BuiltInComponents = options.disallowVue3BuiltInComponents === true;
		const isHtmlElementCaseSensitive = options.htmlElementCaseSensitive === true;
		const isReservedInHtml = (name) => RESERVED_NAMES_IN_HTML.has(name) || !isHtmlElementCaseSensitive && CAPITALIZED_RESERVED_NAMES_IN_HTML.has(name);
		const isReserved = (name) => isReservedInHtml(name) || RESERVED_NAMES_IN_OTHERS.has(name) || !isHtmlElementCaseSensitive && CAPITALIZED_RESERVED_NAMES_IN_OTHERS.has(name) || shouldDisallowVueBuiltInComponents && require_index.default.VUE2_BUILTIN_COMPONENT_NAMES.has(name) || shouldDisallowVue3BuiltInComponents && require_index.default.VUE3_BUILTIN_COMPONENT_NAMES.has(name);
		function getMessageId(name) {
			if (isReservedInHtml(name)) return "reservedInHtml";
			if (require_index.default.VUE2_BUILTIN_COMPONENT_NAMES.has(name)) return "reservedInVue";
			return require_index.default.VUE3_BUILTIN_COMPONENT_NAMES.has(name) ? "reservedInVue3" : "reserved";
		}
		function reportIfInvalid(node) {
			const name = require_index.default.getStringLiteralValue(node);
			if (name !== null && isReserved(name)) report(node, name);
		}
		function report(node, name) {
			context.report({
				node,
				messageId: getMessageId(name),
				data: { name }
			});
		}
		return require_index.default.compositingVisitors(require_index.default.executeOnCallVueComponent(context, (node) => {
			if (node.arguments.length !== 2) return;
			const argument = node.arguments[0];
			if (require_index.default.isStringLiteral(argument)) reportIfInvalid(argument);
		}), require_index.default.executeOnVue(context, (obj) => {
			for (const { node, name } of require_index.default.getRegisteredComponents(obj)) if (isReserved(name)) report(node, name);
			const node = require_index.default.findProperty(obj, "name");
			if (!node || !require_index.default.isStringLiteral(node.value)) return;
			reportIfInvalid(node.value);
		}), require_index.default.defineScriptSetupVisitor(context, { onDefineOptionsEnter(node) {
			const define = node.arguments[0];
			if (define?.type !== "ObjectExpression") return;
			const nameNode = require_index.default.findProperty(define, "name");
			if (!nameNode || !require_index.default.isStringLiteral(nameNode.value)) return;
			reportIfInvalid(nameNode.value);
		} }));
	}
};
exports.default = no_reserved_component_names_default;
