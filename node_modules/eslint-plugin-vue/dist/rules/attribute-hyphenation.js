const require_index = require("../utils/index.js");
const require_casing = require("../utils/casing.js");
const require_regexp = require("../utils/regexp.js");
const require_svg_attributes_weird_case = require("../utils/svg-attributes-weird-case.json.js");
// lib/rules/attribute-hyphenation.ts
/**
* @fileoverview Define a style for the props casing in templates.
* @author Armano
*/
function getAttributeName(node) {
	if (!node.directive) return node.key.rawName;
	return (node.key.name.name === "bind" || node.key.name.name === "model") && node.key.argument && node.key.argument.type === "VIdentifier" ? node.key.argument.rawName : null;
}
var attribute_hyphenation_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce attribute naming style on custom components in template",
			categories: ["vue3-strongly-recommended", "vue2-strongly-recommended"],
			url: "https://eslint.vuejs.org/rules/attribute-hyphenation.html"
		},
		fixable: "code",
		schema: [{ enum: ["always", "never"] }, {
			type: "object",
			properties: {
				ignore: {
					type: "array",
					items: { allOf: [
						{ type: "string" },
						{ not: {
							type: "string",
							pattern: ":exit$"
						} },
						{ not: {
							type: "string",
							pattern: String.raw`^\s*$`
						} }
					] },
					uniqueItems: true
				},
				ignoreTags: {
					type: "array",
					items: { type: "string" },
					uniqueItems: true
				}
			},
			additionalProperties: false
		}],
		messages: {
			mustBeHyphenated: "Attribute '{{text}}' must be hyphenated.",
			cannotBeHyphenated: "Attribute '{{text}}' can't be hyphenated."
		}
	},
	create(context) {
		const sourceCode = context.sourceCode;
		const option = context.options[0];
		const optionsPayload = context.options[1];
		const shouldUseHyphenated = option !== "never";
		const isIgnoredTagName = require_regexp.toRegExpGroupMatcher(optionsPayload?.ignoreTags);
		const ignoredAttributes = [
			"data-",
			"aria-",
			"slot-scope",
			...require_svg_attributes_weird_case.default
		];
		if (optionsPayload && optionsPayload.ignore) ignoredAttributes.push(...optionsPayload.ignore);
		const caseConverter = require_casing.getExactConverter(shouldUseHyphenated ? "kebab-case" : "camelCase");
		function reportIssue(node, name) {
			const text = sourceCode.getText(node.key);
			context.report({
				node: node.key,
				loc: node.loc,
				messageId: shouldUseHyphenated ? "mustBeHyphenated" : "cannotBeHyphenated",
				data: { text },
				fix: (fixer) => text.includes("_") || text.endsWith(".sync") || /^[A-Z]/.test(name) ? null : fixer.replaceText(node.key, text.replace(name, () => caseConverter(name)))
			});
		}
		function isIgnoredAttribute(value) {
			if (ignoredAttributes.some((attr) => value.includes(attr))) return true;
			return shouldUseHyphenated ? value.toLowerCase() === value : !/-/.test(value);
		}
		return require_index.default.defineTemplateBodyVisitor(context, { VAttribute(node) {
			const element = node.parent.parent;
			if (!require_index.default.isCustomComponent(element) && element.name !== "slot" || isIgnoredTagName(element.rawName)) return;
			const name = getAttributeName(node);
			if (name === null || isIgnoredAttribute(name)) return;
			reportIssue(node, name);
		} });
	}
};
exports.default = attribute_hyphenation_default;
