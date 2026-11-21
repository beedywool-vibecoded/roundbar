// lib/utils/ts-utils/typescript.ts
let cacheTypeScript;
/**
* Get TypeScript instance
*/
function getTypeScript() {
	if (cacheTypeScript) return cacheTypeScript;
	try {
		return cacheTypeScript = require("typescript");
	} catch (error) {
		if (error.code === "MODULE_NOT_FOUND") return;
		throw error;
	}
}
/**
* Check if a given type is an object type or not.
*/
function isObject(tsType) {
	const ts = getTypeScript();
	return (tsType.flags & ts.TypeFlags.Object) !== 0;
}
/**
* Check if a given type is an array-like type or not.
*/
function isArrayLikeObject(tsType) {
	const ts = getTypeScript();
	return isObject(tsType) && (tsType.objectFlags & (ts.ObjectFlags.ArrayLiteral | ts.ObjectFlags.EvolvingArray | ts.ObjectFlags.Tuple)) !== 0;
}
/**
* Check if a given type is an any type or not.
*/
function isAny(tsType) {
	const ts = getTypeScript();
	return (tsType.flags & ts.TypeFlags.Any) !== 0;
}
/**
* Check if a given type is an unknown type or not.
*/
function isUnknown(tsType) {
	const ts = getTypeScript();
	return (tsType.flags & ts.TypeFlags.Unknown) !== 0;
}
/**
* Check if a given type is a never type or not.
*/
function isNever(tsType) {
	const ts = getTypeScript();
	return (tsType.flags & ts.TypeFlags.Never) !== 0;
}
/**
* Check if a given type is an null type or not.
*/
function isNull(tsType) {
	const ts = getTypeScript();
	return (tsType.flags & ts.TypeFlags.Null) !== 0;
}
/**
* Check if a given type is a string-like type or not.
*/
function isStringLike(tsType) {
	const ts = getTypeScript();
	return (tsType.flags & ts.TypeFlags.StringLike) !== 0;
}
/**
* Check if a given type is an number-like type or not.
*/
function isNumberLike(tsType) {
	const ts = getTypeScript();
	return (tsType.flags & ts.TypeFlags.NumberLike) !== 0;
}
/**
* Check if a given type is an boolean-like type or not.
*/
function isBooleanLike(tsType) {
	const ts = getTypeScript();
	return (tsType.flags & ts.TypeFlags.BooleanLike) !== 0;
}
/**
* Check if a given type is an bigint-like type or not.
*/
function isBigIntLike(tsType) {
	const ts = getTypeScript();
	return (tsType.flags & ts.TypeFlags.BigIntLike) !== 0;
}
/**
* Check if a given type is a reference type or not.
*/
function isReferenceObject(tsType) {
	const ts = getTypeScript();
	return isObject(tsType) && (tsType.objectFlags & ts.ObjectFlags.Reference) !== 0;
}
/**
* Check if a given type is `function` or not.
*/
function isFunction(tsType) {
	const ts = getTypeScript();
	if (tsType.symbol && (tsType.symbol.flags & (ts.SymbolFlags.Function | ts.SymbolFlags.Method)) !== 0) return true;
	return tsType.getCallSignatures().length > 0;
}
exports.getTypeScript = getTypeScript;
exports.isAny = isAny;
exports.isArrayLikeObject = isArrayLikeObject;
exports.isBigIntLike = isBigIntLike;
exports.isBooleanLike = isBooleanLike;
exports.isFunction = isFunction;
exports.isNever = isNever;
exports.isNull = isNull;
exports.isNumberLike = isNumberLike;
exports.isObject = isObject;
exports.isReferenceObject = isReferenceObject;
exports.isStringLike = isStringLike;
exports.isUnknown = isUnknown;
