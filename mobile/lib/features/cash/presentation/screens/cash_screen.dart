import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../../../../core/widgets/empty_state.dart';

class CashScreen extends StatefulWidget {
  const CashScreen({super.key});

  @override
  State<CashScreen> createState() => _CashScreenState();
}

class _CashScreenState extends State<CashScreen> {
  DateTime? _lastBackPressTime;

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        debugPrint('BACK_DIAG: CASH PopScope invoked didPop=$didPop');
        if (didPop) return;
        final now = DateTime.now();
        if (_lastBackPressTime == null ||
            now.difference(_lastBackPressTime!) > const Duration(seconds: 2)) {
          _lastBackPressTime = now;
          ScaffoldMessenger.of(context).removeCurrentSnackBar();
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text('Tekan sekali lagi untuk keluar dari aplikasi'),
              duration: const Duration(seconds: 2),
            ),
          );
        } else {
          SystemNavigator.pop();
        }
      },
      child: Scaffold(
        appBar: AppBar(title: const Text('Kas')),
        body: const Center(
          child: EmptyState(
            icon: Icons.account_balance_wallet_outlined,
            title: 'Kas',
            description: 'Pengelolaan kas akan tersedia pada tahap berikutnya.',
          ),
        ),
      ),
    );
  }
}
