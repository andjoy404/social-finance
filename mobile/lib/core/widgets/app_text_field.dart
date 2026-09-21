import 'package:flutter/material.dart';

/// Input field wrapper that delegates decoration to the current theme.
class AppTextField extends StatelessWidget {
  final TextEditingController? controller;
  final String? label;
  final String? hintText;
  final String? Function(String?)? validator;
  final TextInputType? keyboardType;
  final bool obscureText;
  final Widget? suffixIcon;
  final AutovalidateMode? autovalidateMode;

  const AppTextField({
    super.key,
    this.controller,
    this.label,
    this.hintText,
    this.validator,
    this.keyboardType,
    this.obscureText = false,
    this.suffixIcon,
    this.autovalidateMode,
  });

  @override
  Widget build(BuildContext context) {
    return TextFormField(
      controller: controller,
      obscureText: obscureText,
      keyboardType: keyboardType,
      autovalidateMode: autovalidateMode ?? AutovalidateMode.disabled,
      validator: validator,
      decoration: InputDecoration(
        labelText: label,
        hintText: hintText,
        suffixIcon: suffixIcon,
      ),
    );
  }
}
