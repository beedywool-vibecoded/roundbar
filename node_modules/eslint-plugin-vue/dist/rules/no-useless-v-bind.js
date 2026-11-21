const require_index = require("../utils/index.js");
// lib/rules/no-useless-v-bind.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
const DOUBLE_QUOTES_RE = /"/gu;
const SINGLE_QUOTES_RE = /'/gu;
var no_useless_v_bind_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow unnecessary `v-bind` directives",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-useless-v-bind.html"
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
		messages: { unexpected: "Unexpected `v-bind` with a string literal value." }
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
			const { value, key } = node;
			if (!value || key.modifiers.length > 0) return;
			const { expression } = value;
			if (!expression || !require_index.default.isStringLiteral(expression)) return;
			const strValue = require_index.default.getStringLiteralValue(expression);
			if (strValue == null) return;
			const rawValue = expression.type === "Literal" ? sourceCode.getText(expression).slice(1, -1) : expression.quasis[0].value.raw;
			const hasComment = sourceCode.parserServices.getTemplateBodyTokenStore().getTokens(value, { includeComments: true }).some((t) => t.type === "Block" || t.type === "Line");
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
				*fix(fixer) {
					if (hasComment || hasEscape) return;
					const quoteChar = sourceCode.getText(value)[0];
					const isShorthand = key.name.rawName === ":";
					const keyDirectiveRange = [key.name.range[0], key.name.range[1] + (isShorthand ? 0 : 1)];
					yield fixer.removeRange(keyDirectiveRange);
					let attrValue;
					if (quoteChar === "\"") {
						attrValue = strValue.replaceAll(DOUBLE_QUOTES_RE, "&quot;");
						attrValue = quoteChar + attrValue + quoteChar;
					} else if (quoteChar === "'") {
						attrValue = strValue.replaceAll(SINGLE_QUOTES_RE, "&apos;");
						attrValue = quoteChar + attrValue + quoteChar;
					} else attrValue = strValue.replaceAll(DOUBLE_QUOTES_RE, "&quot;").replaceAll(SINGLE_QUOTES_RE, "&apos;");
					yield fixer.replaceText(value, attrValue);
				}
			});
		}
		return require_index.default.defineTemplateBodyVisitor(context, { "VAttribute[directive=true][key.name.name='bind'][key.argument!=null]": verify });
	}
};
exports.default = no_useless_v_bind_default;
