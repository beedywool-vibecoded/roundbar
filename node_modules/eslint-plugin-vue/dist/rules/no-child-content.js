const require_index = require("../utils/index.js");
const require_indent_utils = require("../utils/indent-utils.js");
// lib/rules/no-child-content.ts
/**
* @author Flo Edelmann
* See LICENSE file in root directory for full license.
*/
function comparePositions(pos1, pos2) {
	if (pos1.line < pos2.line || pos1.line === pos2.line && pos1.column < pos2.column) return "less";
	return pos1.line > pos2.line || pos1.line === pos2.line && pos1.column > pos2.column ? "greater" : "equal";
}
function getLocationRange(nodes) {
	let start;
	let end;
	for (const node of nodes) {
		if (!start || comparePositions(node.loc.start, start) === "less") start = node.loc.start;
		if (!end || comparePositions(node.loc.end, end) === "greater") end = node.loc.end;
	}
	return start === void 0 || end === void 0 ? void 0 : {
		start,
		end
	};
}
var no_child_content_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow element's child contents which would be overwritten by a directive like `v-html` or `v-text`",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-child-content.html"
		},
		fixable: null,
		hasSuggestions: true,
		schema: [{
			type: "object",
			additionalProperties: false,
			properties: { additionalDirectives: {
				type: "array",
				uniqueItems: true,
				minItems: 1,
				items: { type: "string" }
			} },
			required: ["additionalDirectives"]
		}],
		messages: {
			disallowedChildContent: "Child content is disallowed because it will be overwritten by the v-{{ directiveName }} directive.",
			removeChildContent: "Remove child content."
		}
	},
	create(context) {
		const directives = /* @__PURE__ */ new Set(["html", "text"]);
		const option = context.options[0];
		if (option !== void 0) for (const directive of option.additionalDirectives) directives.add(directive);
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true]"(directiveNode) {
			const directiveName = directiveNode.key.name.name;
			const elementNode = directiveNode.parent.parent;
			if (elementNode.endTag === null || !directives.has(directiveName)) return;
			const elementComments = context.sourceCode.parserServices.getTemplateBodyTokenStore().getTokensBetween(elementNode.startTag, elementNode.endTag, {
				includeComments: true,
				filter: (token) => token.type === "HTMLComment"
			});
			const childNodes = [...elementNode.children, ...elementComments];
			if (childNodes.some((childNode) => require_indent_utils.isNotEmptyTextNode(childNode))) context.report({
				node: elementNode,
				loc: getLocationRange(childNodes),
				messageId: "disallowedChildContent",
				data: { directiveName },
				suggest: [{
					messageId: "removeChildContent",
					*fix(fixer) {
						for (const childNode of childNodes) yield fixer.remove(childNode);
					}
				}]
			});
		} });
	}
};
exports.default = no_child_content_default;
