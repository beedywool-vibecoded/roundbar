const require_index = require("../utils/index.js");
// lib/rules/quote-props.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
var quote_props_default = require_index.default.wrapStylisticOrCoreRule("quote-props", {
	skipDynamicArguments: true,
	preprocess(context, { wrapContextToOverrideProperties, defineVisitor }) {
		const sourceCode = context.sourceCode;
		let htmlQuote = null;
		defineVisitor({
			"VAttribute > VExpressionContainer.value"(node) {
				const firstChar = sourceCode.getText(node)[0];
				htmlQuote = firstChar === "'" || firstChar === "\"" ? firstChar : null;
			},
			"VAttribute > VExpressionContainer.value:exit"() {
				htmlQuote = null;
			}
		});
		wrapContextToOverrideProperties({ report(descriptor) {
			if (htmlQuote) {
				const expectedQuote = htmlQuote === "\"" ? "'" : "\"";
				context.report({
					...descriptor,
					*fix(fixer) {
						const fixes = require_index.default.flatten(descriptor.fix && descriptor.fix(fixer));
						for (const fix of fixes) yield fixer.replaceTextRange(fix.range, fix.text.replaceAll(/["']/gu, () => expectedQuote));
					}
				});
			} else context.report(descriptor);
		} });
	}
});
exports.default = quote_props_default;
