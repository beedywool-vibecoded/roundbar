const require_index = require("../utils/index.js");
// lib/rules/no-useless-mustaches.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
/**
* Strip quotes string
*/
function stripQuotesForHTML(text) {
	if ([
		"\"",
		"'",
		"`"
	].includes(text[0]) && text[0] === text.at(-1)) return text.slice(1, -1);
	const re = /^(?:&(?:quot|apos|#\d+|#x[\da-f]+);|["'`])([\s\S]*)(?:&(?:quot|apos|#\d+|#x[\da-f]+);|["'`])$/u.exec(text);
	return re ? re[1] : null;
}
/** Complete character references such as `&amp;`, `&#38;` and `&#x26;`. */
const CHARACTER_REFERENCE = /&(?:[a-z][\da-z]*|#\d+|#x[\da-f]+);/giu;
/** `&bsol;`, `&#92;` and `&#x5c;` decode to a backslash, which starts an escape sequence. */
const BACKSLASH_REFERENCE = /&(?:bsol|#0*92|#x0*5c);/iu;
/**
* Checks whether the given string literal content contains a character reference
* that would not render the same as static text.
* Vue decodes character references in mustaches before evaluating the expression,
* so only complete references without backslash escapes are kept as they are.
*/
function hasUnsafeCharacterReference(text) {
	if (!text.includes("&")) return false;
	const hasIncompleteReference = text.replaceAll(CHARACTER_REFERENCE, "").includes("&");
	return text.includes("\\") || BACKSLASH_REFERENCE.test(text) || hasIncompleteReference;
}
/**
* Checks whether the text before the given offset ends with an unterminated
* character reference start such as `&`, `&am` or `&#12`.
*/
function isAfterCharacterReferenceStart(text, offset) {
	let index = offset - 1;
	while (index >= 0 && /[\w#]/u.test(text[index])) index--;
	return text[index] === "&";
}
var no_useless_mustaches_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow unnecessary mustache interpolations",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-useless-mustaches.html"
		},
		fixable: "code",
		schema: [{
			type: "object",
			properties: {
				ignoreIncludesComment: { type: "boolean" },
				ignoreStringEscape: { type: "boolean" }
			},
			additionalProperties: false
		}],
		messages: { unexpected: "Unexpected mustache interpolation with a string literal value." }
	},
	create(context) {
		const opts = context.options[0] || {};
		const ignoreIncludesComment = opts.ignoreIncludesComment;
		const ignoreStringEscape = opts.ignoreStringEscape;
		const sourceCode = context.sourceCode;
		/**
		* Report if the value expression is string literals
		*/
		function verify(node) {
			const { expression } = node;
			if (!expression || !require_index.default.isStringLiteral(expression)) return;
			const strValue = require_index.default.getStringLiteralValue(expression);
			if (strValue === null) return;
			const rawValue = expression.type === "Literal" ? sourceCode.getText(expression).slice(1, -1) : expression.quasis[0].value.raw;
			const hasComment = sourceCode.parserServices.getTemplateBodyTokenStore().getTokens(node, { includeComments: true }).some((t) => t.type === "Block" || t.type === "Line");
			if (ignoreIncludesComment && hasComment) return;
			let hasEscape = false;
			if (rawValue !== strValue) {
				const chars = [...rawValue];
				let c = chars.shift();
				while (c) {
					if (c === "\\") {
						c = chars.shift();
						if (c == null || "nrvtbfux".includes(c)) {
							hasEscape = true;
							break;
						}
					}
					c = chars.shift();
				}
			}
			if (ignoreStringEscape && hasEscape) return;
			context.report({
				node,
				messageId: "unexpected",
				fix(fixer) {
					if (hasComment || hasEscape) return null;
					const text = stripQuotesForHTML(sourceCode.getText(expression));
					if (text == null || text.includes("\n") || /^\s|\s$/u.test(text)) return null;
					const replacement = text.replaceAll("<", "&lt;").replaceAll(/\\([\S\s])/g, "$1");
					const charBefore = sourceCode.text[node.range[0] - 1] ?? "";
					const charAfter = sourceCode.text[node.range[1]] ?? "";
					const canStartMustache = `${charBefore}${replacement}${charAfter}`.includes("{{");
					const canJoinCharacterReference = isAfterCharacterReferenceStart(sourceCode.text, node.range[0]) && /^[\w#;]/u.test(`${replacement}${charAfter}`);
					return canStartMustache || canJoinCharacterReference || hasUnsafeCharacterReference(text) ? null : fixer.replaceText(node, replacement);
				}
			});
		}
		return require_index.default.defineTemplateBodyVisitor(context, { "VElement > VExpressionContainer": verify });
	}
};
exports.default = no_useless_mustaches_default;
